
# ai-proxy


## 功能

1. **自定义端口**：通过配置文件设置服务器监听的端口。
2. **API 映射**：通过配置文件定义路径前缀和目标 API 地址的映射关系。
3. **安全头设置**：自动为响应添加安全相关的 HTTP 头。
4. **请求转发**：支持 GET、POST 等 HTTP 方法的请求转发。

---

## 快速开始

### 1.下载二进制可执行文件

## Linux

```bash
wget https://github.com/meowrain/ai-proxy/releases/download/V1.0.0/aiproxy-linux-amd64
chmox +x aiproxy-linux-amd64
```

### 2. 配置

编辑 `config.json` 文件，设置服务器端口和 API 映射规则。以下是一个示例配置：

```bash
vim config.json
```

```json
{
    "port": "8090",
    "api_mapping": {
        "/discord": "https://discord.com/api",
        "/telegram": "https://api.telegram.org",
        "/openai": "https://api.openai.com",
        "/claude": "https://api.anthropic.com",
        "/gemini": "https://generativelanguage.googleapis.com",
        "/meta": "https://www.meta.ai/api",
        "/groq": "https://api.groq.com/openai",
        "/xai": "https://api.x.ai",
        "/cohere": "https://api.cohere.ai",
        "/huggingface": "https://api-inference.huggingface.co",
        "/together": "https://api.together.xyz",
        "/novita": "https://api.novita.ai",
        "/portkey": "https://api.portkey.ai",
        "/fireworks": "https://api.fireworks.ai",
        "/openrouter": "https://openrouter.ai/api"
    }
}
```

- `port`：服务器监听的端口号（例如 `8090`）。
- `api_mapping`：路径前缀和目标 API 地址的映射关系。

### 3. 运行服务器

使用以下命令启动服务器：

```bash
tmux new -s aiproxy
./aiproxy-linux-amd64
```

服务器将启动并监听配置文件中指定的端口。例如，如果端口设置为 `8090`，则服务器将运行在 `http://localhost:8090`。

---

## 从源码编译

```shell
git clone https://github.com/meowrain/ai-proxy.git
make
```

---

## 使用方法

### 1. 访问根路径

访问服务器的根路径（例如 `http://localhost:8090/`），将返回以下响应：

```
Service is running!
```

### 2. 请求转发

服务器会根据 `api_mapping` 中的配置将请求转发到目标 API。例如：

- 请求 `http://localhost:8090/openai/v1/chat/completions` 将被转发到 `https://api.openai.com/v1/chat/completions`。
- 请求 `http://localhost:8090/discord/v10/users/@me` 将被转发到 `https://discord.com/api/v10/users/@me`。

### 3. 自定义端口

修改 `config.json` 文件中的 `port` 字段即可更改服务器端口。例如：

```json
{
    "port": "8080"
}
```

重启服务器后，它将运行在 `http://localhost:8080`。

---

## 配置文件说明

### `config.json`

| 字段        | 类型            | 说明                           |
|-------------|-----------------|--------------------------------|
| `port`      | 字符串          | 服务器监听的端口号（例如 `8090`）。 |
| `api_mapping` | 对象（键值对） | 路径前缀和目标 API 地址的映射关系。 |

---

## 示例

### 示例 1：转发 OpenAI 请求

1. 配置 `config.json`：

   ```json
   {
       "port": "8090",
       "api_mapping": {
           "/openai": "https://api.openai.com"
       }
   }
   ```

2. 启动服务器：

   ```bash
   go run main.go
   ```

3. 发送请求：

   ```bash
   curl -X POST http://localhost:8090/openai/v1/chat/completions \
        -H "Content-Type: application/json" \
        -d '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

   该请求将被转发到 `https://api.openai.com/v1/chat/completions`。

### 示例 2：自定义端口

1. 配置 `config.json`：

   ```json
   {
       "port": "8080",
       "api_mapping": {
           "/discord": "https://discord.com/api"
       }
   }
   ```

2. 启动服务器：

   ```bash
   go run main.go
   ```

3. 发送请求：

   ```bash
   curl http://localhost:8080/discord/v10/users/@me
   ```

   该请求将被转发到 `https://discord.com/api/v10/users/@me`。

---

## 依赖

- Go 1.23 或更高版本。

---

## 许可证

本项目基于 [MIT 许可证](LICENSE) 开源。

---

## 反馈与贡献

如有任何问题或建议，请提交 Issue 或 Pull Request。

---

## 作者

- [MeowRain](https://github.com/meowrain)

---

Enjoy! 🚀