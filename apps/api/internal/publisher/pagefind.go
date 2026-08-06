package publisher

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

// runPagefindIndex 在构建产物上生成 Pagefind 静态搜索索引(写入 <sitePath>/pagefind/)。
// 二进制缺失或索引失败以 error 返回,由调用方降级为警告——发布不因搜索索引失败而中断。
func runPagefindIndex(ctx context.Context, cfg config.Config, sitePath string) (string, error) {
	bin := cfg.PagefindBin
	if _, err := os.Stat(bin); err != nil {
		lookedUp, lookErr := exec.LookPath("pagefind")
		if lookErr != nil {
			return "", fmt.Errorf("pagefind binary not found at %s or in PATH", cfg.PagefindBin)
		}
		bin = lookedUp
	}

	cmd := exec.CommandContext(ctx, bin, "--site", sitePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pagefind index failed: %w: %s", err, compactLogOutput(output))
	}
	return compactLogOutput(output), nil
}
