# Go AI Proxy Server

一个灵活的Go语言实现的API代理服务器，支持基于路径前缀的请求转发，并允许为每个API映射或全局配置HTTP/SOCKS5代理。

## 特性

*   基于路径前缀的动态请求转发。
*   支持为每个API目标单独配置HTTP或SOCKS5代理。
*   支持全局配置HTTP或SOCKS5代理作为默认选项。
*   灵活的`api.json`配置文件。
*   支持通过Docker部署。

## 运行
```bash
docker run -d \
  --add-host=host.docker.internal:$(ip -4 addr show docker0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}') \
  -p 8094:8094 \
  ai-proxy:latest
```
## 配置文件 (`api.json`)

`api.json` 文件用于定义代理服务器的监听端口、API路由映射以及代理设置。

```json
{
  "port": "8090",
  "proxy": {
    "type": "socks5",
    "url": "host.docker.internal:2080" 
  },
  "api_mapping": {
    "/serviceA": "http://target-service-a.com",
    "/serviceB": {
      "target_url": "https://target-service-b.com/api",
      "proxy": {
        "type": "http",
        "address": "http://specific-proxy.com:8888"
      }
    },
    "/serviceC": {
      "target_url": "https://target-service-c.com",
      "proxy": null 
    },
    "/anotherService": {
        "target_url": "http://another-target.com"
    }
  }
}
```

**配置项说明:**

*   `port` (字符串, 必填): 代理服务器监听的端口号。
*   `proxy` (对象, 可选): 全局代理配置。如果配置了此项，所有未特殊指定代理的API请求将默认通过此代理。
    *   `type` (字符串, 必填): 代理类型，可选值为 `"http"` 或 `"socks5"`。
    *   `address` (字符串, 可选): 代理服务器地址 (例如 `"http://proxy.example.com:8080"` 或 `"localhost:1080"`)。优先使用此字段。
    *   `url` (字符串, 可选): 代理服务器地址的另一种形式。如果 `address` 字段为空，则使用此字段。主要用于全局代理配置，便于Docker环境下指向宿主机代理 (如 `"host.docker.internal:1080"`)。
*   `api_mapping` (对象, 必填): 定义API路径前缀到目标服务的映射。
    *   **键** (字符串): API的路径前缀 (例如 `"/serviceA"`)。
    *   **值** (字符串或对象):
        *   **字符串**: 直接指定目标服务的基础URL (例如 `"http://target-service-a.com"`)。这种形式的请求会使用全局代理（如果已配置）。
        *   **对象**: 更详细的配置，包含以下字段：
            *   `target_url` (字符串, 必填): 目标服务的基础URL。
            *   `proxy` (对象或null, 可选): 为此特定API配置的代理。其结构与全局`proxy`对象相同。 
                *   如果提供，则此API请求将使用这里定义的代理，覆盖全局代理设置。
                *   如果为 `null`，则此API请求将 **不使用任何代理**，即使已配置全局代理。（*请根据实际实现确认此行为，当前代码是：如果特定代理为`nil`或`null`，则会尝试使用全局代理。如果希望`null`能强制不使用全局代理，需要调整代码逻辑*）
                *   如果不提供此`proxy`字段，则此API请求将尝试使用全局代理（如果已配置）。

## 本地运行

### 前提条件

*   Go 1.22 或更高版本。
*   项目根目录下有 `api.json` 配置文件。

### 步骤

1.  克隆仓库 (如果适用) 或将代码下载到本地。
2.  安装依赖：
    ```bash
    go mod tidy
    go get golang.org/x/net/proxy # 如果尚未获取
    ```
3.  运行服务器：
    ```bash
    go run main.go
    ```
    服务器将在 `api.json` 中指定的端口上启动。

## 使用 Docker 部署

### 构建镜像

确保 `Dockerfile` 和 `api.json` 文件位于项目根目录。

```bash
# 标准构建
docker build -t go-ai-proxy .
```

### 运行容器

```bash
# 基本运行，将容器的8090端口映射到主机的8090端口
docker run -p 8094:8094 go-ai-proxy
```
*(请将`8090`替换为`api.json`中配置的实际端口)*

#### 使容器内应用使用宿主机代理

如果容器内运行的Go应用需要通过宿主机的代理服务器访问外部网络，请按以下方式操作：

1.  **修改 `api.json`**:
    将 `api.json` 中需要走代理的 `proxy` 配置的 `address` 或 `url` 字段设置为指向宿主机。
    *   对于 Docker Desktop (Mac/Windows)，通常可以使用 `host.docker.internal`。
        例如，全局代理配置为宿主机的SOCKS5代理 `127.0.0.1:2080`：
        ```json
        {
          "proxy": {
            "type": "socks5",
            "url": "host.docker.internal:2080"
          },
          // ...
        }
        ```
2.  **运行容器**:
    *   **Docker Desktop (Mac/Windows):**
        ```bash
        docker run -p 8090:8094 go-ai-proxy
        ```
    *   **Linux (Docker 18.03+):**
        为了使 `host.docker.internal` 在Linux上生效，需要添加 `--add-host` 参数：
        ```bash
        docker run \
        --add-host=host.docker.internal:$(ip -4 addr show docker0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}') \
        -p 8094:8094 \
        ai-proxy
        ```
    *   **Linux (备选方案，共享宿主机网络 - 牺牲隔离性):**
        ```bash
        docker run --network="host" go-ai-proxy
        ```
        在此模式下，`api.json` 中的代理地址可以直接使用 `127.0.0.1:宿主机代理端口`。

## 开发

(此处可添加关于代码结构、如何贡献等信息)

## 许可证

MIT