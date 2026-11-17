// Package i18n holds InkFerry's Chinese and English UI copy.
package i18n

import "strings"

// Lang is a UI language.
type Lang string

const (
	ZH Lang = "zh-cn"
	EN Lang = "en-us"
)

// Messages is one catalogue.
type Messages map[string]string

var zh = Messages{
	"输出格式":      "输出格式",
	"拖放文件":      "拖放文件到这里，或点击选择",
	"支持格式提示":    "EPUB · MOBI · AZW3 · AZW · PDF · DOCX · TXT",
	"正在选择文件":    "正在选择文件…",
	"等待添加文件":    "准备好转换下一本书",
	"转换中":       "转换中",
	"已保存":       "已保存：",
	"不支持的文件":    "文件格式不支持",
	"已跳过不支持的文件": "已添加支持的文件，其余文件已跳过",
	"转码任务列表":    "转码任务列表",
	"终止":        "终止",
	"定位":        "定位",
	"开始":        "开始",
	"等待":        "等待",
	"失败":        "失败",
	"运行日志":      "运行日志",
	"关于":        "关于",
	"转码失败提示":    "很遗憾，转换没有完成",
	"失败原因解释":    "这次转换未完成，常见原因包括：",
	"失败原因1":     "1. 源文件损坏、加密，或不符合该格式的结构规范",
	"失败原因2":     "2. 源格式与目标格式差异过大，当前转换方式无法完成",
	"失败原因3":     "3. 转换被中断，或目标目录不可写",
	"失败后做法":     "可以先重试一次；若仍然失败，请换一个与源文件更接近的目标格式。若源文件在其他阅读器中也无法打开，通常需要先修复源文件。",
	"错误信息输出":    "诊断信息",
	"下载组件":      "下载组件 - ",
	"组件安装完成":    "组件安装完成",
	"检测组件":      "正在检查可选组件",
	"组件说明":      "EPUB、PDF、DOCX 与 TXT 可直接转换。MOBI / AZW3 输出需要可选的 Calibre 组件。",
	"请求失败":      "下载失败，请在网络恢复后重试",
	"版本过低标题":    "请升级到最新版本",
	"版本过低内容":    "当前版本已停止支持。请下载最新版 InkFerry 后重试。",
	"前往下载":      "前往下载新版本",
	"给您带来不便":    "给您带来不便，请谅解",
	"已完成":       "已完成",
	"已终止":       "已终止",
	"进度":        "进度",
	"选择文件":      "选择电子书文件",
	"清空":        "清空已完成",
	"安装Calibre": "安装 Calibre",
	"关于说明":      "轻量、原生的电子书格式转换器，界面由 Go 直接绘制。",
	"最小化":       "最小化",
	"最大化或还原":    "最大化 / 还原",
	"关闭窗口":      "关闭",
}

var en = Messages{
	"输出格式":      "Output format",
	"拖放文件":      "Drop files here or click to browse",
	"支持格式提示":    "EPUB · MOBI · AZW3 · AZW · PDF · DOCX · TXT",
	"正在选择文件":    "Choosing files…",
	"等待添加文件":    "Ready for your next book",
	"转换中":       "Converting",
	"已保存":       "Saved: ",
	"不支持的文件":    "Unsupported file format",
	"已跳过不支持的文件": "Supported files added; other files skipped",
	"转码任务列表":    "Task list",
	"终止":        "Cancel",
	"定位":        "Show in folder",
	"开始":        "Start",
	"等待":        "Queued",
	"失败":        "Failed",
	"运行日志":      "Logs",
	"关于":        "About",
	"转码失败提示":    "The conversion did not finish",
	"失败原因解释":    "A conversion can stop for several reasons:",
	"失败原因1":     "1. The source is damaged, encrypted, or not a valid file of its format",
	"失败原因2":     "2. The source and target formats are too different for this converter",
	"失败原因3":     "3. The conversion was interrupted, or the destination is not writable",
	"失败后做法":     "Try once more. If it still fails, choose a target format closer to the source. If the source cannot be opened in another reader either, it probably needs repairing first.",
	"错误信息输出":    "Diagnostics",
	"下载组件":      "Downloading - ",
	"组件安装完成":    "Component installed",
	"检测组件":      "Checking the optional component",
	"组件说明":      "EPUB, PDF, DOCX and TXT convert directly. MOBI and AZW3 output use the optional Calibre component.",
	"请求失败":      "Download failed; retry when the connection is back",
	"版本过低标题":    "Please upgrade",
	"版本过低内容":    "This version is no longer supported. Download the latest InkFerry and try again.",
	"前往下载":      "Download the latest release",
	"给您带来不便":    "Sorry for the inconvenience",
	"已完成":       "Done",
	"已终止":       "Stopped",
	"进度":        "Progress",
	"选择文件":      "Choose ebook files",
	"清空":        "Clear finished",
	"安装Calibre": "Install Calibre",
	"关于说明":      "A small native ebook converter, with its interface drawn directly by Go.",
	"最小化":       "Minimize",
	"最大化或还原":    "Maximize / Restore",
	"关闭窗口":      "Close",
}

var current = ZH

// Set picks the language.
func Set(l Lang) { current = l }

// Current reports the language in use.
func Current() Lang { return current }

// Detect picks zh-cn for Chinese system locales, en-us otherwise.
func Detect(tag string) Lang {
	t := strings.ToLower(tag)
	if strings.HasPrefix(t, "zh") {
		return ZH
	}
	return EN
}

// T looks a message up in the language in use.
func T(key string) string { return Of(current, key) }

// Of looks a message up in l, falling back to Chinese and then to the key.
func Of(l Lang, key string) string {
	m := zh
	if l == EN {
		m = en
	}
	if v, ok := m[key]; ok {
		return v
	}
	if v, ok := zh[key]; ok {
		return v
	}
	return key
}
