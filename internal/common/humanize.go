package common

import (
	"fmt"
	"math"
	"time"
)

func BytesIEC(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(b) / float64(div)
	suffix := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}[exp]
	if value >= 100 {
		return fmt.Sprintf("%.0f %s", value, suffix)
	}
	if value >= 10 {
		return fmt.Sprintf("%.1f %s", value, suffix)
	}
	return fmt.Sprintf("%.2f %s", value, suffix)
}

func BytesSI(b uint64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(b) / float64(div)
	suffix := []string{"KB", "MB", "GB", "TB", "PB"}[exp]
	if value >= 100 {
		return fmt.Sprintf("%.0f %s", value, suffix)
	}
	if value >= 10 {
		return fmt.Sprintf("%.1f %s", value, suffix)
	}
	return fmt.Sprintf("%.2f %s", value, suffix)
}

func Percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return (float64(used) / float64(total)) * 100
}

func Clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func FormatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalHours := int64(d.Hours())
	days := totalHours / 24
	hours := totalHours % 24
	minutes := int64(d.Minutes()) % 60
	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%d天%d小时", days, hours)
		}
		return fmt.Sprintf("%d天", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%d小时%d分", hours, minutes)
		}
		return fmt.Sprintf("%d小时", hours)
	}
	return fmt.Sprintf("%d分", minutes)
}

