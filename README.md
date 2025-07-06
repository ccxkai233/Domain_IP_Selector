# 域名优选 IP 引擎 (Domain IP Selector)

## 🚀 项目概述

本工具旨在通过对大量域名进行 DNS 解析，筛选出 Cloudflare 的 IP 地址，并对这些 IP 进行延迟和下载速度测试，最终根据地理位置和测试结果，筛选出在当前网络环境下表现最佳的 IP 地址。

## ✨ 主要特性

- **零依赖运行**: 无需任何外部文件，下载单个可执行文件即可运行。首次启动时会自动生成所需的配置文件和数据文件。
- **IP 版本选择**: 可通过配置选择测试 IPv4 或 IPv6。
- **智能缓存**: 自动缓存 Cloudflare 的 IP 段，不同版本的 IP 使用独立缓存，避免重复下载。
- **域名去重**: 自动去除域名列表中的重复项，提高解析效率。
- **多维度测试**:
  - **延迟测试**: 使用 HTTPing 测试 IP 的延迟和丢包率。
  - **速度测试**: 对低延迟的 IP 进行真实下载速度测试。
- **区域化筛选**: 按地理区域（如亚洲、北美等）对 IP 进行分组，并选出各区域表现最好的 Top N 个 IP。
- **全面并发控制**: 可分别配置 DNS 解析、延迟测试、速度测试的并发数，以平衡测试速度和系统资源占用。
- **格式化输出**: 将最终结果同时输出为 `.json` 和 `.csv` 格式，并根据 IP 版本自动命名。

## 📂 项目结构

```
Domain_IP_Selector_Go/
├── cmd/
│   └── main.go           # 程序主入口
├── internal/
│   ├── config/
│   │   └── config.go     # 配置加载逻辑
│   ├── datasource/
│   │   ├── cfips.go      # Cloudflare IP 数据源处理
│   │   └── domains.go    # 域名列表数据源处理
│   ├── locations/
│   │   └── locations.go  # IP 地理位置映射
│   ├── output/
│   │   ├── csv.go        # CSV 文件输出
│   │   └── json.go       # JSON 文件输出
│   └── tester/
│       ├── httping.go    # 延迟测试
│       ├── speedtest.go  # 速度测试
│       └── utils.go      # 测试相关工具函数
├── pkg/
│   └── model/
│       └── types.go      # 项目核心数据结构 (未创建，但建议)
├── go.mod
├── go.sum
└── README.md
```

## ⚙️ 工作流程

程序遵循以下步骤来完成优选过程：

1.  **加载配置**: 从项目根目录的 `config.yaml` 文件加载配置。
2.  **初始化数据源**:
    - 加载 `locations.json` 获取 Cloudflare Colo 数据中心到地理区域的映射。
    - 加载 `reputation_domains.txt` 获取用于解析的域名列表（已去重）。
    - 根据配置的 IP 版本，加载或下载对应的 Cloudflare IP 段（例如 `cf-ips-ipv4.txt`）。
3.  **IP 筛选与延迟测试**:
    - **并发解析**所有域名（并发数可控），根据配置只查找 IPv4 或 IPv6 地址。
    - 对所有解析出的 IP 地址进行**去重**，确保每个 IP 只被测试一次。
    - 将去重后的 IP 与 Cloudflare IP 段进行比对，过滤出属于 **Cloudflare 的 IP**。
    - 对所有 Cloudflare IP 进行**并发 HTTPing 延迟测试**（并发数可控）。
    - 淘汰掉延迟过高或丢包率过高的 IP。
4.  **分组与 Top N 筛选**:
    - 将通过延迟测试的 IP 按地理区域分组。
    - 在每个区域内，按延迟从低到高排序，选出前 N 个（N 可配置）IP 进入下一轮。
5.  **下载速度测试**:
    - 对上一轮筛选出的所有 IP 进行并发下载速度测试。
    - 使用信号量机制控制并发数，防止因带宽抢占导致结果不准。
6.  **生成结果**:
    - 将包含延迟、丢包率、下载速度等信息的最终结果按速度从高到低排序。
    - 将结果写入到带 IP 版本标识的 `result_ipvX.json` 和 `result_ipvX.csv` 文件中。

## 🛠️ 如何使用

### 1. 运行程序

直接双击项目根目录下的 `DomainIPSelector.exe` 文件。

**首次运行**:
程序在启动时会检查所需文件是否存在。如果 `config.yaml`, `locations.json`, 或 `reputation_domains.txt` 任何一个文件缺失，程序会自动在 `.exe` 文件旁边创建一份默认版本。

### 2. 修改配置 (可选)

首次运行后，您可以根据需要编辑自动生成的 `config.yaml` 文件：

```yaml
# --- 并发配置 ---
# dns_concurrency: 用于 DNS 解析的最大并发数。
# 建议值: 30。
dns_concurrency: 30

# latency_test_concurrency: 用于延迟测试的最大并发数。
# 建议值: 10。设置过高可能触发目标服务器的速率限制。
latency_test_concurrency: 10

# speedtest_concurrency: 用于下载速度测试的最大并发数。
# 建议值：1-3。设置过高可能因抢占带宽导致测速不准。
speedtest_concurrency: 1

# --- 筛选配置 ---
# max_latency: 延迟测试中允许的最大延迟（单位：毫秒）。
# 超过此延迟的 IP 将被直接淘汰。
max_latency: 300

# top_n_per_region: 从每个地理区域中，选择延迟最低的前 N 个 IP 进入最终的速度测试。
top_n_per_region: 5

# --- IP 版本配置 ---
# ip_version: 选择要测试的 IP 版本。
# 可选值："ipv4" 或 "ipv6"。默认为 "ipv4"。
ip_version: ipv4
```

### 3. 查看结果

程序运行完毕后，会在 `.exe` 文件所在的目录下生成结果文件，例如 `result_ipv4.json` 和 `result_ipv4.csv`。

### 4. 如何编译与分发

#### 编译

如果需要自行编译，请确保您已安装 Go 语言环境。然后打开终端，进入 `Domain_IP_Selector_Go` 目录，执行以下命令：

```bash
# -s -w 参数可以减小生成文件的大小
go build -ldflags "-s -w" -o "..\DomainIPSelector.exe" ./cmd
```

该命令会在项目根目录（上一级目录）生成 `DomainIPSelector.exe` 文件。

#### 分发

得益于文件内嵌技术，您现在**只需分发编译好的 `DomainIPSelector.exe` 单个文件**即可。


## 📝 更新日志

- **零依赖运行**: 将所有必需的配置文件 (`config.yaml`, `locations.json`, `reputation_domains.txt`) 内嵌到可执行文件中，实现单文件分发和首次运行自动生成。
- **全面并发控制**: 为 DNS 解析和延迟测试增加了独立的并发数量控制，有效避免因请求过快导致的测试失败。
- **IP 版本分离**: 实现了 IPv4 / IPv6 的可配置测试模式。
- **缓存与输出优化**: 为不同 IP 版本使用独立的缓存文件，并生成带版本标识的结果文件。
- **域名去重**: 在加载时自动去除重复的域名，提高解析效率。
- **IP 地址去重**: 在解析后自动去除重复的 IP 地址，避免冗余测试。
- **路径修复**: 修正了文件相对路径问题，使程序能正确启动。
- **文档完善**: 创建并持续更新了详细的 README 文档。

## 🤝 如何贡献与二次开发

欢迎对本项目进行贡献！以下是一些二次开发的建议：

- **增加测试维度**: 可以增加 TCPing、丢包率等更详细的网络质量测试。
- **完善数据模型**: 在 `pkg/model/` 目录下建立更完善的数据模型，使代码更清晰。
- **错误处理**: 对网络请求和文件操作增加更健壮的重试和错误处理逻辑。
- **命令行参数**: 将配置项（如并发数、延迟阈值等）改为通过命令行参数传入，增加灵活性。
- **Web 界面**: 为工具开发一个简单的 Web 界面，使其更易于使用。

如果您有任何想法或问题，欢迎提交 Issue 或 Pull Request。

## 🙏 致谢与许可

本项目的核心测速逻辑和部分代码实现，受到了 [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) 项目的启发并直接使用了其部分源代码。我们对原作者的辛勤工作和无私分享表示衷心的感谢。

根据原项目的开源协议，本项目同样基于 **GNU General Public License v3.0 (GPL-3.0)** 进行分发。您可以在项目根目录找到 `LICENSE` 文件的副本。如果您分发此软件或其衍生版本，您必须遵守此协议的条款。