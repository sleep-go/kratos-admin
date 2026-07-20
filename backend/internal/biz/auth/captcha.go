package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
	"time"
)

const captchaTTL = 5 * time.Minute

var (
	// ErrCaptchaInvalid 表示图形验证码不存在、已过期或答案错误。
	ErrCaptchaInvalid = errors.New("图形验证码错误或已过期")
)

// CaptchaStore 定义一次性图形验证码摘要存储。
type CaptchaStore interface {
	Put(ctx context.Context, id, answerHash string, ttl time.Duration) error
	Take(ctx context.Context, id string) (string, error)
}

// CaptchaChallenge 描述图形验证码标识与 PNG Data URI。
type CaptchaChallenge struct {
	ID       string
	ImageURI string
	ExpireAt time.Time
}

// CaptchaUsecase 生成并校验一次性数字图形验证码。
type CaptchaUsecase struct {
	store CaptchaStore
	now   func() time.Time
}

// NewCaptchaUsecase 创建图形验证码用例。
func NewCaptchaUsecase(store CaptchaStore, now func() time.Time) *CaptchaUsecase {
	if now == nil {
		now = time.Now
	}
	return &CaptchaUsecase{store: store, now: now}
}

// Generate 生成五位数字验证码并保存摘要五分钟。
func (u *CaptchaUsecase) Generate(ctx context.Context) (CaptchaChallenge, error) {
	id, err := randomTokenID()
	if err != nil {
		return CaptchaChallenge{}, err
	}
	answer, err := randomDigits(5)
	if err != nil {
		return CaptchaChallenge{}, err
	}
	if err := u.store.Put(ctx, id, captchaHash(id, answer), captchaTTL); err != nil {
		return CaptchaChallenge{}, err
	}
	imageURI, err := renderCaptcha(answer)
	if err != nil {
		return CaptchaChallenge{}, err
	}
	return CaptchaChallenge{ID: id, ImageURI: imageURI, ExpireAt: u.now().UTC().Add(captchaTTL)}, nil
}

// Verify 消费并校验一次性验证码，错误答案同样立即失效。
func (u *CaptchaUsecase) Verify(ctx context.Context, id, answer string) error {
	if id == "" || answer == "" {
		return ErrCaptchaInvalid
	}
	want, err := u.store.Take(ctx, id)
	if err != nil || want == "" || want != captchaHash(id, strings.TrimSpace(answer)) {
		return ErrCaptchaInvalid
	}
	return nil
}

func captchaHash(id, answer string) string {
	sum := sha256.Sum256([]byte(id + ":" + answer))
	return hex.EncodeToString(sum[:])
}

func randomDigits(length int) (string, error) {
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	var result strings.Builder
	for _, value := range raw {
		result.WriteByte('0' + value%10)
	}
	return result.String(), nil
}

var digitSegments = [10]string{"abcdef", "bc", "abdeg", "abcdg", "bcfg", "acdfg", "acdefg", "abc", "abcdefg", "abcdfg"}

func renderCaptcha(answer string) (string, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 150, 48))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 245, G: 245, B: 245, A: 255}}, image.Point{}, draw.Src)
	for i, character := range answer {
		digit, _ := strconv.Atoi(string(character))
		drawDigit(canvas, 12+i*27, 7, digit, color.RGBA{R: 47, G: 47, B: 47, A: 255})
	}
	noise := make([]byte, 24)
	_, _ = rand.Read(noise)
	for i := 0; i < len(noise); i += 4 {
		x := int(noise[i]) % canvas.Bounds().Dx()
		y := int(noise[i+1]) % canvas.Bounds().Dy()
		width := 2 + int(noise[i+2])%8
		draw.Draw(canvas, image.Rect(x, y, min(x+width, 150), min(y+1, 48)), &image.Uniform{C: color.RGBA{R: 225, G: 45, B: 45, A: 90}}, image.Point{}, draw.Over)
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

func drawDigit(canvas draw.Image, x, y, digit int, ink color.Color) {
	segments := map[byte]image.Rectangle{
		'a': image.Rect(x+4, y, x+17, y+3), 'b': image.Rect(x+17, y+3, x+20, y+16),
		'c': image.Rect(x+17, y+19, x+20, y+32), 'd': image.Rect(x+4, y+32, x+17, y+35),
		'e': image.Rect(x+1, y+19, x+4, y+32), 'f': image.Rect(x+1, y+3, x+4, y+16),
		'g': image.Rect(x+4, y+16, x+17, y+19),
	}
	for _, segment := range []byte(digitSegments[digit]) {
		draw.Draw(canvas, segments[segment], &image.Uniform{C: ink}, image.Point{}, draw.Src)
	}
}
