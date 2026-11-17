# Changelog

All notable changes to InkFerry are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and releases use
[Semantic Versioning](https://semver.org/).

## [1.0.1] - 2026-11-17

### Fixed

- Windows 图形界面版不再弹出多余的控制台窗口：GUI 二进制改用 `windowsgui` 子系统构建。

### Changed

- 发行版改为便携版：每个平台两个自包含可执行文件，`-gui` 是桌面版，`-cli` 是命令行版，不再打包 ZIP / tar.gz。
- 发行版不再附带 README、更新日志、许可证副本、截图和 `ferryinstall`。
- 程序不再写配置文件、日志或缓存：转换临时文件放在系统临时目录并在完成后删除，只有主动安装 Calibre 组件时才会创建 `%APPDATA%\InkFerry`。

## [1.0.0] - 2026-11-17

### Added

- Native desktop converter for EPUB, MOBI, AZW, AZW3, PDF, DOCX and TXT.
- Batch queue with three concurrent conversions, cancellation, progress and per-task logs.
- Frameless window with custom minimize, maximize and close controls.
- Chinese and English interface with light and dark themes.
- `ferry` command-line converter and `ferryinstall` optional component installer.
- Safe output naming that never overwrites existing files.
- Cross-platform release packaging for Windows x64/ARM64, Linux x64 and macOS Apple Silicon.
- Automated GitHub Actions CI and release publishing.
