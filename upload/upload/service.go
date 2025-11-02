package upload

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// 正确写法：把注释放在函数上方
// encore:api public method=POST path=/upload
func Upload(ctx context.Context, req *UploadRequest) (*UploadResponse, error) {
	// 1. 解码BASE64内容
	fileBytes, err := base64.StdEncoding.DecodeString(req.Content)
	if err != nil {
		return &UploadResponse{Success: false, Message: "文件内容解码失败"}, nil
	}

	// 2. 初始化 MinIO 客户端
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		return &UploadResponse{Success: false, Message: "MinIO连接失败"}, nil
	}

	bucketName := "uploads" // 你需要提前在MinIO控制台创建好这个bucket

	// 3. 上传文件到 MinIO
	reader := bytes.NewReader(fileBytes)
	_, err = minioClient.PutObject(ctx, bucketName, req.Filename, reader, int64(len(fileBytes)), minio.PutObjectOptions{})
	if err != nil {
		return &UploadResponse{Success: false, Message: fmt.Sprintf("上传失败: %v", err)}, nil
	}

	return &UploadResponse{Success: true, Message: "上传成功"}, nil
}

// 类型定义放在函数外，且不要加 encore:api 注释
type UploadRequest struct {
	Filename string // 文件名
	Content  string // 文件内容，为BASE64字符串
}

type UploadResponse struct {
	Success bool
	Message string
}
