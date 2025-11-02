package media

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// 视频上传接口：接收multipart/form-data格式的视频文件
// encore:api public method=POST path=/upload-video
func UploadVideo(ctx context.Context, req *VideoUploadRequest) (*UploadResponse, error) {
	// 1. 基本校验：检查文件是否存在
	if req.File == nil {
		return &UploadResponse{Success: false, Message: "未上传视频文件"}, nil
	}

	// 2. 打开文件流（获取io.Reader）
	file, err := req.File.Open()
	if err != nil {
		return &UploadResponse{Success: false, Message: "无法读取视频文件"}, nil
	}
	defer file.Close() // 确保文件流最终关闭

	// 3. 视频文件校验（可选：根据业务需求调整）
	// 3.1 限制文件大小（示例：最大100MB）
	maxSize := int64(1024 * 1024 * 1024) // 100MB
	if req.File.Size > maxSize {
		return &UploadResponse{
			Success: false,
			Message: fmt.Sprintf("文件过大，最大支持%dMB", maxSize/(1024*1024)),
		}, nil
	}

	// 3.2 校验MIME类型（示例：只允许常见视频格式）
	allowedMimeTypes := map[string]bool{
		"video/mp4":  true,
		"video/mpeg": true,
		"video/avi":  true,
		"video/mov":  true,
		"video/webm": true,
	}
	if !allowedMimeTypes[req.File.Header.Get("Content-Type")] {
		return &UploadResponse{
			Success: false,
			Message: "不支持的视频格式，仅允许mp4、mpeg、avi、mov、webm",
		}, nil
	}

	// 4. 初始化MinIO客户端（生产环境建议通过配置文件读取参数）
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false, // 生产环境建议开启HTTPS（true）
	})
	if err != nil {
		return &UploadResponse{Success: false, Message: "MinIO客户端初始化失败"}, nil
	}

	// 5. 定义存储桶和文件名（使用原文件名，可根据需求添加前缀或UUID防重名）
	bucketName := "media" // 需提前在MinIO创建此存储桶
	filename := req.File.Filename
	// 可选：添加时间戳或UUID避免文件名重复，例如：
	// filename = fmt.Sprintf("%d_%s", time.Now().Unix(), req.File.Filename)

	// 6. 上传视频到MinIO（自动分块处理大文件）
	_, err = minioClient.PutObject(ctx, bucketName, filename, file, req.File.Size, minio.PutObjectOptions{
		ContentType: req.File.Header.Get("Content-Type"), // 保留原视频MIME类型
	})
	if err != nil {
		return &UploadResponse{
			Success: false,
			Message: fmt.Sprintf("上传失败: %v", err),
		}, nil
	}

	// 7. 上传成功，返回结果（可添加视频访问URL）
	videoURL := fmt.Sprintf("http://localhost:9000/%s/%s", bucketName, filename) // 需确保MinIO允许公开访问
	return &UploadResponse{
		Success: true,
		Message: fmt.Sprintf("视频上传成功，访问地址: %s", videoURL),
	}, nil
}

// 视频上传请求参数（multipart/form-data格式）
type VideoUploadRequest struct {
	// File字段对应前端表单的name="file"，Encore会自动解析multipart文件
	File *multipart.FileHeader `json:"file"` // 视频文件流
}

// 上传响应结果
type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
