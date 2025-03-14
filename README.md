# VPSub

[![Go Report Card](https://goreportcard.com/badge/github.com/djx30103/vpsub)](https://goreportcard.com/report/github.com/djx30103/vpsub)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/djx30103/vpsub)](https://github.com/djx30103/vpsub/releases)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/djx30103/vpsub)](https://github.com/djx30103/vpsub)
[![GitHub stars](https://img.shields.io/github/stars/djx30103/vpsub)](https://github.com/djx30103/vpsub/stargazers)
[![License](https://img.shields.io/github/license/djx30103/vpsub)](https://github.com/djx30103/vpsub/blob/main/LICENSE)

一个轻量级的VPS流量监控工具，帮助自建节点用户实时掌握各节点的流量使用情况。通过获取VPS服务商的流量数据并注入到订阅文件中，让你在使用代理客户端时直观地了解每个节点的流量状态。

## ✨ 核心特性

### 🔄 流量管理
- **实时监控**: 获取VPS流量使用数据，包括已用流量、剩余流量、总流量和重置时间
- **订阅集成**: 自动将流量信息注入到订阅文件，支持在代理软件中直观显示

### 🛠 系统特性
- **多账户管理**: 支持多个VPS账号和订阅文件的统一管理
- **高效缓存**: 多级缓存机制，智能避免API限速
- **容器部署**: 支持Docker容器化部署，便于维护和迁移
- **多服务商**: 支持多种VPS服务商API，持续扩展中

# VPSub
## 📊 支持的服务商

| <div align="center">服务商</div> | <div align="center">流量查询</div> | <div align="center">重置日期</div> | <div align="center">配置参数映射</div> |
|:-------:|:---------:|:---------:|:-------------:|
| BandwagonHost | ✅ | ✅ | `api_id`: VEID<br>`api_key`: API KEY |
| 更多服务商 | 🔄 | 🔄 | 敬请期待 |

</div>

## 🔍 工作原理

VPSub 通过以下步骤处理每个订阅请求：
```
1. 读取配置文件中的VPS服务商API凭证
2. 调用相应服务商的API获取流量使用情况
3. 读取订阅文件内容
4. 将流量信息注入到HTTP响应头中
5. 返回订阅文件内容，同时包含流量信息
```

## 🚀 快速开始

### 1. 安装部署

#### 方式一：直接运行

```bash
# 克隆仓库
git clone https://github.com/djx30103/vpsub.git
cd vpsub

# 直接运行
go run cmd/server/main.go

# 或者构建后运行
go build -o vpsub cmd/server/main.go
./vpsub
```

#### 方式二：Docker部署

```bash
docker run -d \
  --name vpsub \
  -p 30103:30103 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/subscriptions:/app/subscriptions \
  ghcr.io/djx30103/vpsub:latest
```

#### 方式三：使用Docker Compose

```yaml
services:
  vpsub:
    image: ghcr.io/djx30103/vpsub:latest
    container_name: vpsub
    ports:
      - "30103:30103"
    volumes:
      - ./data/config:/app/config
      - ./data/subscriptions:/app/subscriptions
    restart: unless-stopped
    environment:
      TZ: Asia/Shanghai
```

运行：

```bash
docker-compose up -d
```

### 2. 准备订阅文件

将你的代理配置文件放入`subscriptions`目录。

### 3. 修改配置文件

编辑`config/config.yml`文件，添加你的API凭证和订阅文件信息：

```yaml
# 应用模式：release、debug（默认release）
app_mode: release

# 服务器配置
server:
  # 监听地址和端口
  listen_addr: :30103
  # 请求超时时间
  timeout: 30s

# 日志配置
log:
  # 日志级别: debug, info, warn, error
  level: warn

# 存储配置
storage:
  # 订阅文件存储主目录
  subscription_dir: ./subscriptions

# 默认配置（可被服务商配置覆盖）
defaults:
  # 缓存配置
  cache:
    # 文件缓存时间，0表示不缓存（文件内容缓存的话，修改后的文件需要等待缓存失效）
    file_ttl: 30s
    # API响应缓存时间，0表示不缓存（API接口建议缓存，避免触发服务商的限速）
    api_ttl: 60s
    # 最终响应缓存时间，0表示不缓存（文件内容 + api响应结果）
    response_ttl: 60s

  # 服务商通用配置
  provider:
    # API请求超时时间
    request_timeout: 10s
    # 数据更新间隔
    update_interval: 24h

# VPS服务商配置
providers:
  # 搬瓦工(BandwagonHost)配置
  bandwagonhost:
    # API路由前缀（必须唯一，不同账号不能使用相同的路由前缀）
    - route_prefix: "/your-custom-path"
      # API凭证
      api_id: "your-veid"           # 填入搬瓦工控制面板中的VEID
      api_key: "your-api-key"       # 填入搬瓦工控制面板中的API KEY
      # 订阅文件
      subscriptions:
        - "your-subscription-file.yaml"

    # 可以配置多个账号
    - route_prefix: "/another-custom-path"
      api_id: "another-veid"
      api_key: "another-api-key"
      subscriptions:
        - "another-subscription-file.yaml"
      # 覆盖默认配置（可选）
      overrides:
        cache:
          file_ttl: 30s
          api_ttl: 60s
          response_ttl: 0
        provider:
          request_timeout: 10s
          update_interval: 24h

  # Vultr配置模板（即将支持）
  # vultr:
  #   - route_prefix: "/vultr-custom-path"
  #     api_key: "your-vultr-api-key"
  #     subscriptions:
  #       - "vultr-subscription-file.yaml"
```

### 4. 使用订阅链接

#### 订阅链接格式

```
http://your-server:30103/<route_prefix>/<subscription_file>
```

#### 参数说明

- `your-server`: 你的服务器地址
- `route_prefix`: 配置文件中设置的路由前缀
- `subscription_file`: 订阅文件名称（例如：config.yaml）

#### 示例

如果你的配置如下：
```yaml
providers:
  bandwagonhost:
    - route_prefix: "/bwh01"
      subscriptions:
        - "my-proxy.yaml"
```

那么你的订阅链接就是：
```
http://your-server:30103/bwh01/my-proxy.yaml
```

#### 注意事项

- 每个`route_prefix`必须是唯一的，不同账号不能使用相同的路由前缀
- 订阅链接末尾的文件名必须与配置文件中的`subscriptions`列表中的文件名完全匹配
- 确保你的服务器和端口（默认30103）可以正常访问

将生成的订阅链接添加到你的代理客户端即可使用。


## 📄 许可证

本项目采用MIT许可证，详见[LICENSE](LICENSE)文件。
