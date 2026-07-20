package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type localObjectMeta struct {
	ContentType string
	Size        int64
	Metadata    map[string]string
}

// LocalProvider 使用短期 HMAC URL 在本地目录提供受限上传和下载。
type LocalProvider struct {
	root     string
	basePath string
	secret   []byte
	now      func() time.Time
}

// NewLocalProvider 创建本地文件 Provider。
func NewLocalProvider(root, basePath string, secret []byte, now func() time.Time) (*LocalProvider, error) {
	if root == "" || basePath == "" {
		return nil, errors.New("本地存储目录和访问路径不能为空")
	}
	if len(secret) < 32 {
		return nil, errors.New("本地存储签名密钥至少需要32字节")
	}
	if now == nil {
		now = time.Now
	}
	return &LocalProvider{root: filepath.Clean(root), basePath: basePath, secret: append([]byte(nil), secret...), now: now}, nil
}

// Name 返回 Provider 稳定名称。
func (p *LocalProvider) Name() string { return "local" }

// PresignUpload 生成仅允许写入指定对象和大小的短期 URL。
func (p *LocalProvider) PresignUpload(_ context.Context, objectKey string, meta ObjectMeta, ttl time.Duration) (SignedRequest, error) {
	if _, err := p.objectPath(objectKey); err != nil {
		return SignedRequest{}, err
	}
	return p.sign(http.MethodPut, objectKey, "", meta, ttl), nil
}

// Head 返回本地对象大小和上传时保存的元数据。
func (p *LocalProvider) Head(_ context.Context, objectKey string) (ObjectMeta, error) {
	filePath, err := p.objectPath(objectKey)
	if err != nil {
		return ObjectMeta{}, err
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		return ObjectMeta{}, err
	}
	raw, err := os.ReadFile(filePath + ".meta.json")
	if err != nil {
		return ObjectMeta{}, err
	}
	var saved localObjectMeta
	if err := json.Unmarshal(raw, &saved); err != nil {
		return ObjectMeta{}, err
	}
	return ObjectMeta{ContentType: saved.ContentType, Size: stat.Size(), Metadata: saved.Metadata}, nil
}

// PresignDownload 生成受限本地下载 URL。
func (p *LocalProvider) PresignDownload(_ context.Context, objectKey, downloadName string, ttl time.Duration) (SignedRequest, error) {
	if _, err := p.objectPath(objectKey); err != nil {
		return SignedRequest{}, err
	}
	return p.sign(http.MethodGet, objectKey, downloadName, ObjectMeta{}, ttl), nil
}

// Delete 删除对象及其元数据文件；对象已不存在时保持幂等。
func (p *LocalProvider) Delete(_ context.Context, objectKey string) error {
	filePath, err := p.objectPath(objectKey)
	if err != nil {
		return err
	}
	for _, target := range []string{filePath, filePath + ".meta.json"} {
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// ServeHTTP 执行经 HMAC 校验的本地上传或下载。
func (p *LocalProvider) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	objectKey := query.Get("key")
	size, _ := strconv.ParseInt(query.Get("size"), 10, 64)
	meta := ObjectMeta{ContentType: query.Get("content_type"), Size: size}
	if metadata := query.Get("metadata"); metadata != "" {
		_ = json.Unmarshal([]byte(metadata), &meta.Metadata)
	}
	if err := p.verify(request.Method, objectKey, query.Get("name"), meta, query.Get("exp"), query.Get("sig")); err != nil {
		http.Error(response, "签名无效或已过期", http.StatusForbidden)
		return
	}
	switch request.Method {
	case http.MethodPut:
		p.serveUpload(response, request, objectKey, meta)
	case http.MethodGet:
		p.serveDownload(response, request, objectKey, query.Get("name"))
	default:
		response.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (p *LocalProvider) serveUpload(response http.ResponseWriter, request *http.Request, objectKey string, meta ObjectMeta) {
	if meta.Size < 0 {
		http.Error(response, "文件大小无效", http.StatusBadRequest)
		return
	}
	filePath, err := p.objectPath(objectKey)
	if err != nil {
		http.Error(response, "对象路径无效", http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		http.Error(response, "创建存储目录失败", http.StatusInternalServerError)
		return
	}
	object, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		if os.IsExist(err) {
			http.Error(response, "对象已存在", http.StatusConflict)
			return
		}
		http.Error(response, "创建对象文件失败", http.StatusInternalServerError)
		return
	}
	written, copyErr := io.Copy(object, io.LimitReader(request.Body, meta.Size+1))
	closeErr := object.Close()
	if copyErr != nil || closeErr != nil || written != meta.Size {
		_ = os.Remove(filePath)
		http.Error(response, "上传内容大小不匹配", http.StatusBadRequest)
		return
	}
	raw, _ := json.Marshal(localObjectMeta{ContentType: meta.ContentType, Size: written, Metadata: meta.Metadata})
	if err := os.WriteFile(filePath+".meta.json", raw, 0o640); err != nil {
		_ = os.Remove(filePath)
		http.Error(response, "保存文件元数据失败", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (p *LocalProvider) serveDownload(response http.ResponseWriter, request *http.Request, objectKey, downloadName string) {
	filePath, err := p.objectPath(objectKey)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	meta, err := p.Head(request.Context(), objectKey)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	if meta.ContentType != "" {
		response.Header().Set("Content-Type", meta.ContentType)
	}
	if downloadName != "" {
		response.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadName}))
	}
	http.ServeFile(response, request, filePath)
}

func (p *LocalProvider) sign(method, objectKey, name string, meta ObjectMeta, ttl time.Duration) SignedRequest {
	expiresAt := p.now().UTC().Add(ttl)
	metadata, _ := json.Marshal(meta.Metadata)
	values := url.Values{
		"key": []string{objectKey}, "name": []string{name}, "content_type": []string{meta.ContentType},
		"size": []string{strconv.FormatInt(meta.Size, 10)}, "metadata": []string{string(metadata)},
		"exp": []string{strconv.FormatInt(expiresAt.Unix(), 10)},
	}
	values.Set("sig", p.signature(method, values))
	return SignedRequest{Method: method, URL: p.basePath + "?" + values.Encode(), ExpiresAt: expiresAt}
}

func (p *LocalProvider) verify(method, objectKey, name string, meta ObjectMeta, expires, signature string) error {
	expiresUnix, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || !time.Unix(expiresUnix, 0).After(p.now().UTC()) {
		return errors.New("签名已过期")
	}
	metadata, _ := json.Marshal(meta.Metadata)
	values := url.Values{
		"key": []string{objectKey}, "name": []string{name}, "content_type": []string{meta.ContentType},
		"size": []string{strconv.FormatInt(meta.Size, 10)}, "metadata": []string{string(metadata)}, "exp": []string{expires},
	}
	expected, decodeErr := hex.DecodeString(signature)
	if decodeErr != nil || !hmac.Equal(expected, mustDecodeHex(p.signature(method, values))) {
		return errors.New("签名不匹配")
	}
	_, err = p.objectPath(objectKey)
	return err
}

func (p *LocalProvider) signature(method string, values url.Values) string {
	mac := hmac.New(sha256.New, p.secret)
	_, _ = io.WriteString(mac, method+"\n"+values.Encode())
	return hex.EncodeToString(mac.Sum(nil))
}

func mustDecodeHex(value string) []byte {
	decoded, _ := hex.DecodeString(value)
	return decoded
}

func (p *LocalProvider) objectPath(objectKey string) (string, error) {
	if objectKey == "" || strings.Contains(objectKey, "\\") || path.Clean("/"+objectKey) != "/"+objectKey {
		return "", fmt.Errorf("对象键不安全")
	}
	target := filepath.Join(p.root, filepath.FromSlash(objectKey))
	relative, err := filepath.Rel(p.root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("对象键越界")
	}
	return target, nil
}

var _ Provider = (*LocalProvider)(nil)
var _ http.Handler = (*LocalProvider)(nil)
