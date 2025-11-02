package media

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

func init() {
	var err error
	minioClient, err = minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		panic(fmt.Sprintf("MinIO 初始化失败: %v", err))
	}
}

// UploadResponse 响应结构体
type UploadResponse struct {
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
	Success bool   `json:"success"`
}

// 支持的视频格式
var supportedVideoFormats = map[string]bool{
	".mp4":  true,
	".avi":  true,
	".mov":  true,
	".mkv":  true,
	".webm": true,
	".flv":  true,
	".wmv":  true,
	".mpeg": true,
	".mpg":  true,
}

// 使用 raw HTTP 端点处理视频上传
// encore:api public raw method=POST path=/video-upload
func UploadVideo(w http.ResponseWriter, req *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// 处理预检请求
	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 检查请求方法
	if req.Method != "POST" {
		sendErrorResponse(w, "只支持 POST 请求", http.StatusMethodNotAllowed)
		return
	}

	// 检查内容类型
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		sendErrorResponse(w, "请求必须是 multipart/form-data 类型", http.StatusBadRequest)
		return
	}

	// 解析 multipart 表单，限制大小为 100MB（视频文件通常较大）
	err := req.ParseMultipartForm(100 << 20) // 100MB
	if err != nil {
		sendErrorResponse(w, "解析表单失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 获取文件
	file, header, err := req.FormFile("video")
	if err != nil {
		sendErrorResponse(w, "获取视频文件失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 检查文件扩展名
	fileExt := strings.ToLower(filepath.Ext(header.Filename))
	if !supportedVideoFormats[fileExt] {
		sendErrorResponse(w,
			fmt.Sprintf("不支持的视频格式: %s。支持的格式: %v", fileExt, getSupportedFormats()),
			http.StatusBadRequest)
		return
	}

	// 检查文件大小（可选，额外验证）
	if header.Size > 100<<20 { // 100MB
		sendErrorResponse(w, "文件大小不能超过 100MB", http.StatusBadRequest)
		return
	}

	// 上传到 MinIO
	bucketName := "videos"
	objectName := generateObjectName(header.Filename)

	// 确保存储桶存在
	err = ensureBucketExists(req.Context(), bucketName)
	if err != nil {
		sendErrorResponse(w, "存储桶创建失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 上传视频文件
	_, err = minioClient.PutObject(
		req.Context(),
		bucketName,
		objectName,
		file,
		header.Size,
		minio.PutObjectOptions{
			ContentType: getVideoContentType(fileExt),
			UserMetadata: map[string]string{
				"original-filename": header.Filename,
				"file-size":         fmt.Sprintf("%d", header.Size),
			},
		},
	)
	if err != nil {
		sendErrorResponse(w, "视频上传失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	videoURL := fmt.Sprintf("http://localhost:9000/%s/%s", bucketName, objectName)
	response := UploadResponse{
		Message: "视频上传成功",
		URL:     videoURL,
		Success: true,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// 辅助函数：发送错误响应
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := UploadResponse{
		Message: message,
		Success: false,
	}
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// 辅助函数：获取支持的格式列表
func getSupportedFormats() []string {
	formats := make([]string, 0, len(supportedVideoFormats))
	for format := range supportedVideoFormats {
		formats = append(formats, format)
	}
	return formats
}

// 辅助函数：生成对象名称（避免文件名冲突）
func generateObjectName(originalName string) string {
	ext := filepath.Ext(originalName)
	name := strings.TrimSuffix(originalName, ext)
	timestamp := fmt.Sprintf("%d", http.TimeFormat) // 简化示例，实际应该用时间戳
	return fmt.Sprintf("video-%s-%s%s", name, timestamp, ext)
}

// 辅助函数：获取视频内容类型
func getVideoContentType(fileExt string) string {
	contentTypes := map[string]string{
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".mkv":  "video/x-matroska",
		".webm": "video/webm",
		".flv":  "video/x-flv",
		".wmv":  "video/x-ms-wmv",
		".mpeg": "video/mpeg",
		".mpg":  "video/mpeg",
	}
	if contentType, ok := contentTypes[fileExt]; ok {
		return contentType
	}
	return "application/octet-stream"
}

// 辅助函数：确保存储桶存在
func ensureBucketExists(ctx context.Context, bucketName string) error {
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return err
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
		// 设置存储桶策略为公开读取（可选）
		policy := `{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {"AWS": ["*"]},
					"Action": ["  s3:GetObject"],
					"Resource": ["arn:aws:s3:::` + bucketName + `/*"]
				}
			]
		}`
		err = minioClient.SetBucketPolicy(ctx, bucketName, policy)
		if err != nil {
			return err
		}
	}
	return nil
}
