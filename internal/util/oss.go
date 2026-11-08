package util

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"

	"StudioAttendance_backend/internal/config"
)

// ossClient 全局 OSS 客户端,InitOSS 之后可用
var ossClient *oss.Client

// InitOSS 初始化 OSS 客户端,main 启动时调用一次。
// .env 里 OSS_REGION / OSS_BUCKET / OSS_ACCESS_KEY_ID 任一为空就报错,
// main 里可以选择只打日志不退出(OSS 不可用但服务照常跑)。
func InitOSS() error {
	if config.OSSRegion == "" || config.OSSBucket == "" || config.OSSAccessKeyID == "" {
		return fmt.Errorf("OSS 未配置(检查 .env 中的 OSS_* 配置项)")
	}

	cfg := oss.LoadDefaultConfig().
		WithRegion(config.OSSRegion).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.OSSAccessKeyID, config.OSSAccessKeySecret))

	// Endpoint 可选:留空时 SDK 会用 Region 推导默认公网地址
	if config.OSSEndpoint != "" {
		cfg = cfg.WithEndpoint(config.OSSEndpoint)
	}

	ossClient = oss.NewClient(cfg)
	return nil
}

// Upload 上传一个文件,返回它的公开访问 URL。
//
//	dir          对象目录前缀,如 "avatars"、"schedules"
//	originalName 原始文件名,用于保留扩展名(决定浏览器如何解析 Content-Type)
//	reader       文件内容;gin 里用 fileHeader.Open() 打开后传入
func Upload(ctx context.Context, dir, originalName string, reader io.Reader) (string, error) {
	if ossClient == nil {
		return "", fmt.Errorf("OSS 未初始化,请先调用 InitOSS")
	}

	objectKey := genObjectKey(dir, originalName)

	_, err := ossClient.PutObject(ctx, &oss.PutObjectRequest{
		Bucket: oss.Ptr(config.OSSBucket),
		Key:    oss.Ptr(objectKey),
		Body:   reader,
	})
	if err != nil {
		return "", fmt.Errorf("上传 OSS 失败: %w", err)
	}

	return publicURL(objectKey), nil
}

// SignURL 生成临时签名访问链接。
// Bucket 权限为"私有"时,公开 URL 会被拒,用这个生成限时链接给前端。
func SignURL(ctx context.Context, objectKey string, expire time.Duration) (string, error) {
	if ossClient == nil {
		return "", fmt.Errorf("OSS 未初始化,请先调用 InitOSS")
	}

	result, err := ossClient.Presign(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(config.OSSBucket),
		Key:    oss.Ptr(objectKey),
	}, oss.PresignExpires(expire))
	if err != nil {
		return "", fmt.Errorf("生成签名 URL 失败: %w", err)
	}

	return result.URL, nil
}

// genObjectKey 生成不重复的对象名:avatars/20261108153012_a1b2c3d4.jpg
// 时间戳保证大致有序,随机串防止同一秒内并发上传互相覆盖
func genObjectKey(dir, originalName string) string {
	ext := strings.ToLower(path.Ext(originalName))

	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		// 随机数失败时退化为纯纳秒时间戳,冲突概率也极低
		return fmt.Sprintf("%s/%d%s", dir, time.Now().UnixNano(), ext)
	}

	return fmt.Sprintf("%s/%s_%s%s",
		dir, time.Now().Format("20060102150405"), hex.EncodeToString(buf), ext)
}

// publicURL 拼对象的公开访问地址(要求 Bucket 为公共读)
func publicURL(objectKey string) string {
	if config.OSSEndpoint != "" {
		ep := strings.TrimPrefix(strings.TrimPrefix(config.OSSEndpoint, "https://"), "http://")
		return fmt.Sprintf("https://%s.%s/%s", config.OSSBucket, ep, objectKey)
	}
	return fmt.Sprintf("https://%s.oss-%s.aliyuncs.com/%s", config.OSSBucket, config.OSSRegion, objectKey)
}
