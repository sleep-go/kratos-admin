// Package storage 提供本地文件与阿里云 OSS 的统一对象存储能力。
package storage

import bizstorage "github.com/sleep-go/kratos-admin/app/admin/internal/biz/storage"

// ObjectMeta 描述上传前约定及上传后校验的对象元数据。
type ObjectMeta = bizstorage.ObjectMeta

// SignedRequest 描述客户端可在短时间内执行的受限对象请求。
type SignedRequest = bizstorage.SignedRequest

// Provider 定义对象存储上传、校验、下载与删除能力。
type Provider = bizstorage.Provider
