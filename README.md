# Domain IP Selector (Go) - Cloudflare 优选IP工具

<p align="center">
  <img src="https://img.shields.io/badge/language-Go-blue.svg" alt="Language">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
  <img src="https://img.shields.io/badge/status-active-brightgreen.svg" alt="Status">
</p>

<p align="center">
  一个用 Go 语言编写的高性能 Cloudflare IP 地址优选工具，旨在帮助用户快速找到当前网络环境下连接质量最佳的 Cloudflare IP。
</p>

---

## 📚 目录

- [Domain IP Selector (Go) - Cloudflare 优选IP工具](#domain-ip-selector-go---cloudflare-优选ip工具)
  - [📚 目录](#-目录)
  - [✨ 主要特性](#-主要特性)
  - [🚀 如何使用](#-如何使用)
    - [🌐 方式一：Web UI 模式 (推荐)](#-方式一web-ui-模式-推荐)
    - [💻 方式二：命令行 (CLI) 模式](#-方式二命令行-cli-模式)
  - [⚙️ 配置文件说明 (`config.yaml`)](#️-配置文件说明-configyaml)
  - [📊 结果文件说明](#-结果文件说明)
  - [🛠️ 如何编译与分发](#️-如何编译与分发)
    - [编译](#编译)
  - [📝 更新日志](#-更新日志)
    - [最新更新 (2025-01-24)](#最新更新-2025-01-24)
  - [致谢](#致谢)

---

## ✨ 主要特性

*   🧠 **核心思路**：使用信誉域名进行DNS，快速获得有潜力的IP，关于什么是信誉域名，以及本项目具体的思路，具体请看[这篇文章](https://github.com/ccxkai233/PublicDocuments/blob/main/Domain%20IP%20Selector%E7%9A%84%E8%AE%BE%E8%AE%A1%E6%80%9D%E8%B7%AF.md)
*   **🖥️ 可视化操作界面**：提供直观的 Web UI，所有操作均可在浏览器中完成，并实时显示优选进度和结果。
*   **⌨️ 双模式支持**：除了推荐的 Web 模式，也为高级用户保留了传统的命令行（CLI）运行模式。
*   **🔧 可配置**：通过网页，您可以自由定制延迟上限、速度上下限、IP版本（IPv4/IPv6）、筛选区域等参数，并一键保存为 `config.yaml` 文件，如果您有高级需求，也可以直接编辑配置文件，里面有更多的可配置项。
*   **💾 结果保存**：优选任务完成后，结果会自动保存为 `result_*.json` 和 `result_*.csv` 文件，方便查看和使用。

## 🚀 如何使用

### 🌐 方式一：Web UI 模式 (推荐)

这是最适合小白上手的使用方式。

1.  🖱️ **双击运行**：直接双击 `main.exe` 文件。
2.  🌐 **自动打开浏览器**：程序会自动在您的默认浏览器中打开操作界面 (地址通常是 `http://localhost:8080`)。
3.  ⚙️ **配置与运行**：
    *   在网页上，您可以直观地修改各项配置参数，根据自己的需要进行调整，也可以一键保存为新的配置文件，方便未来使用。
    *   点击“开始”按钮，程序便会开始执行IP优选任务。
    *   页面下方的日志窗口会实时显示当前的进度。
    *   任务完成后，会自动拖动页面到结果处表格，可以方便的复制优选IP。

> 💡 **提示**: 首次运行程序时，会自动在 `.exe` 文件同目录下生成 `config.yaml`, `locations.json`, `reputation_domains.txt` 三个文件。

### 💻 方式二：命令行 (CLI) 模式

如果您是大佬，更喜欢命令行的方式，或需要在自动化脚本中使用，可以选择此模式。

1.  ⌨️ 打开一个终端（如 PowerShell 或 CMD）。
2.  📂 进入 `main.exe` 所在的目录。
3.  ▶️ 执行以下命令：
    ```bash
    .\main.exe --cli
    ```
4.  📄 程序将会在终端中输出实时日志，并执行优选任务。
5.  🐍 Releases 中附带一个定时优选IP并更新到A记录的python脚本，您可以直接使用，或参考开发自己的脚本。

## ⚙️ 配置文件说明 (`config.yaml`)

您可以通过修改 `config.yaml` 文件来调整优选策略。以下是几个常用参数的说明：

| 参数名              | 说明                                                               | 示例值                  |
| ------------------- | ------------------------------------------------------------------ | ----------------------- |
| `max_latency`       | **最高延迟 (毫秒)**。延迟高于此值的IP会被淘汰。                    | `300`                   |
| `min_speed`         | **最低下载速度 (MB/s)**。速度低于此值的IP会被淘汰。                | `5.0`                   |
| `top_n_per_group`   | **每组保留的IP数**。按区域分组后，每组保留N个最快的IP。            | `5`                     |
| `ip_version`        | **IP版本**。可以设置为 `"ipv4"` 或 `"ipv6"`。                        | `"ipv4"`                |
| `filter_regions`    | **区域筛选**。只测试指定区域的IP，留空则测试所有。                 | `["Asia Pacific", "North America"]`      |
| `filter_colos`      | **Colo筛选**。只测试指定数据中心的IP，留空则测试所有。             | `["SJC", "LAX"]`        |

## 📊 结果文件说明

任务完成后，您会在 `.exe` 目录找到给人类看的 `result_ipv4.csv` (或 `result_ipv6.csv`) 文件。您可以用 Excel 或其他表格软件打开它。

以及给自动化程序看的 `result_ipv4.json` (或 `result_ipv6.json`) 文件。

文件中的关键列说明：

| 列名            | 说明                                       |
| --------------- | ------------------------------------------ |
| `Address`       | 优选出的 Cloudflare IP 地址。              |
| `Delay`         | 该 IP 的网络延迟（单位：毫秒）。           |
| `DownloadSpeed` | 下载速度（单位：KB/s）。                   |
| `Colo`          | 该 IP 所属的 Cloudflare 数据中心代码。     |
| `Region`        | 该 IP 所属的地理区域（如 `North America`）。        |

---

## 🛠️ 如何编译与分发
### 编译
如果需要自行编译，请确保您已安装 Go 语言环境。然后打开终端，进入 Domain_IP_Selector_Go 目录，执行以下命令：

```bash
go build -ldflags "-s -w" -o "..\DomainIPSelector.exe" ./cmd
```
该命令会在项目根目录（上一级目录）生成 DomainIPSelector.exe 文件。

---

## 📝 更新日志

### 最新更新 (2025-01-24)

**🔧 优化测速稳定性**
- **移除备用测速地址切换机制**：删除了当连续10次测速结果不达标时自动切换到备用测速地址的逻辑
- **统一使用稳定测速地址**：现在程序将始终使用 Cloudflare 官方测速地址 (`https://speed.cloudflare.com/__down?bytes=200000000`)，确保测速结果的一致性和可靠性
- **简化错误处理**：当IP测速结果低于最低速度要求时，程序会直接舍弃该IP并继续测试，不再进行复杂的计数和地址切换操作
- **提升性能**：移除了不必要的计数器和锁操作，提高了测速过程的执行效率

> 💡 **说明**：此次更新移除了可能导致测速结果不一致的备用地址机制，专注于使用最稳定的测速源，为用户提供更可靠的IP优选结果。

---

##  致谢

*   感谢 **Gemini大善人** 对本项目的大力支持。ps：如果不是它太过谄媚，好几次把我带沟里去了，这个项目可以完成的快上不少。
*   感谢 [**XIU2/CloudflareSpeedTest**](https://github.com/XIU2/CloudflareSpeedTest) 项目，本项目的测速逻辑来源于此，并直接应用了部分源代码。
*   因此本项目也遵循相同的开源协议 [MIT License](https://github.com/ccxkai233/Domain_IP_Selector/blob/main/LICENSE)。