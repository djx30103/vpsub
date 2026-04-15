package handler

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	emoji "github.com/Andrew-M-C/go.emoji"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/djx30103/vpsub/internal/config"
	"github.com/djx30103/vpsub/pkg/bytesize"
	"github.com/djx30103/vpsub/pkg/provider/base"
)

// createProxyGroupNode 用于构造一个仅承载展示信息的 Clash 分组节点。
// 参数含义：name 为分组展示名称。
// 返回值：返回可直接插入 YAML 语法树的分组节点。
func createProxyGroupNode(name string) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "type"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "select"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "proxies"},
			{
				Kind: yaml.SequenceNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "REJECT"},
				},
			},
		},
	}
}

// newAppendGroupNodes 用于根据流量信息生成需要附加到订阅中的展示分组节点。
// 参数含义：apiInfo 为流量信息；usageDisplay 为展示格式配置。
// 返回值：返回待追加的分组节点列表和模板渲染错误。
func newAppendGroupNodes(apiInfo *base.APIResponseInfo, usageDisplay *config.UsageDisplayConfig) ([]*yaml.Node, error) {
	groupList := make([]*yaml.Node, 0, 2)

	// 重置时间按与配置校验一致的模板规则展开，避免校验与运行时语义不一致。
	resetTimeFormat, err := renderResetTimeUsageTemplate(apiInfo.Expire, usageDisplay.ResetTimeFormat)
	if err != nil {
		return nil, fmt.Errorf("failed to render reset time usage template: %w", err)
	}
	groupList = append(groupList, createProxyGroupNode(resetTimeFormat))

	// 流量展示同样按模板执行，支持条件与内置模板函数等标准语义。
	trafficFormat, err := renderTrafficUsageTemplate(apiInfo, usageDisplay)
	if err != nil {
		return nil, fmt.Errorf("failed to render traffic usage template: %w", err)
	}
	groupList = append(groupList, createProxyGroupNode(trafficFormat))

	return groupList, nil
}

// renderResetTimeUsageTemplate 用于将重置时间按展示模板渲染为订阅分组名称。
// 参数含义：expireUnix 为重置时间戳；format 为配置中的重置时间模板。
// 返回值：返回渲染后的展示文案和模板执行错误。
func renderResetTimeUsageTemplate(expireUnix int64, format string) (string, error) {
	t := time.Unix(expireUnix, 0)

	return config.ExecuteTemplate(format, map[string]string{
		"year":  t.Format("2006"),
		"month": t.Format("01"),
		"day":   t.Format("02"),
	})
}

// renderTrafficUsageTemplate 用于将流量信息按展示模板渲染为订阅分组名称。
// 参数含义：apiInfo 为流量信息；usageDisplay 为展示配置。
// 返回值：返回渲染后的展示文案和模板执行错误。
func renderTrafficUsageTemplate(apiInfo *base.APIResponseInfo, usageDisplay *config.UsageDisplayConfig) (string, error) {
	return config.ExecuteTemplate(usageDisplay.TrafficFormat, map[string]string{
		"used":  bytesize.Format(apiInfo.Download+apiInfo.Upload, usageDisplay.TrafficUnit),
		"total": bytesize.Format(apiInfo.Total, usageDisplay.TrafficUnit),
	})
}

// appendUsageGroups 用于把流量信息组装成代理分组后写回订阅内容。
// 参数含义：fileContent 为原始订阅文件内容；apiInfo 为流量信息；usageDisplay 为展示格式配置。
// 返回值：返回处理后的订阅内容和处理错误。
func appendUsageGroups(fileContent []byte, apiInfo *base.APIResponseInfo, usageDisplay *config.UsageDisplayConfig) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(fileContent, &root); err != nil {
		return nil, fmt.Errorf("failed to read yaml config: %w", err)
	}

	emojiTokens := make(map[string]string)
	replaceNodeEmojiToToken(&root, emojiTokens)

	groupList, err := findProxyGroupsNode(&root)
	if err != nil {
		return nil, err
	}
	if len(groupList.Content) == 0 {
		return nil, errors.New("no proxy-groups found in config")
	}

	appendGroupList, err := newAppendGroupNodes(apiInfo, usageDisplay)
	if err != nil {
		return nil, err
	}
	if len(appendGroupList) == 0 {
		return nil, errors.New("no usage groups found in config")
	}
	for _, groupNode := range appendGroupList {
		replaceNodeEmojiToToken(groupNode, emojiTokens)
	}

	if usageDisplay.Prepend {
		groupList.Content = append(appendGroupList, groupList.Content...)
	} else {
		groupList.Content = append(groupList.Content, appendGroupList...)
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return nil, fmt.Errorf("failed to marshal yaml config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to close yaml encoder: %w", err)
	}

	return restoreEmojiTokens(buffer.Bytes(), emojiTokens), nil
}

// findProxyGroupsNode 用于从 YAML 根节点中定位 proxy-groups 对应的序列节点。
// 参数含义：root 为完整 YAML 文档根节点。
// 返回值：返回 proxy-groups 的序列节点；不存在或结构非法时返回错误。
func findProxyGroupsNode(root *yaml.Node) (*yaml.Node, error) {
	if root == nil || len(root.Content) == 0 {
		return nil, errors.New("yaml document is empty")
	}

	mappingNode := root.Content[0]
	if mappingNode.Kind != yaml.MappingNode {
		return nil, errors.New("yaml root must be mapping")
	}

	// 根节点按 key/value 交替存储，这里只定位 proxy-groups 对应的值节点，避免改动其他结构。
	for i := 0; i < len(mappingNode.Content); i += 2 {
		keyNode := mappingNode.Content[i]
		valueNode := mappingNode.Content[i+1]
		if keyNode.Value != "proxy-groups" {
			continue
		}
		if valueNode.Kind != yaml.SequenceNode {
			return nil, errors.New("proxy-groups must be sequence")
		}
		return valueNode, nil
	}

	return nil, errors.New("no proxy-groups found in config")
}

// replaceNodeEmojiToToken 用于在 YAML 节点树中将 emoji 替换为临时占位符，避免编码器输出 Unicode 转义。
// 参数含义：node 为当前节点；emojiTokens 为 emoji 到占位符的映射表。
// 返回值：无。
func replaceNodeEmojiToToken(node *yaml.Node, emojiTokens map[string]string) {
	if node == nil {
		return
	}

	if node.Kind == yaml.ScalarNode {
		node.Value = replaceEmojiWithToken(node.Value, emojiTokens)
	}

	node.HeadComment = replaceEmojiWithToken(node.HeadComment, emojiTokens)
	node.LineComment = replaceEmojiWithToken(node.LineComment, emojiTokens)
	node.FootComment = replaceEmojiWithToken(node.FootComment, emojiTokens)

	// 递归遍历整棵语法树，确保新增节点和原始节点都走同一套占位符策略。
	for _, child := range node.Content {
		replaceNodeEmojiToToken(child, emojiTokens)
	}
}

// replaceEmojiWithToken 用于把字符串中的 emoji 替换为可逆占位符。
// 参数含义：input 为待处理字符串；emojiTokens 为 emoji 到占位符的映射表。
// 返回值：返回替换后的字符串。
func replaceEmojiWithToken(input string, emojiTokens map[string]string) string {
	if input == "" {
		return input
	}

	return emoji.ReplaceAllEmojiFunc(input, func(current string) string {
		if token, ok := emojiTokens[current]; ok {
			return token
		}

		token := "{{.%EMOJI%}}" + uuid.NewString() + "{{.%EMOJI%}}"
		emojiTokens[current] = token
		return token
	})
}

// restoreEmojiTokens 用于在 YAML 编码完成后恢复占位符对应的 emoji。
// 参数含义：content 为编码后的 YAML 内容；emojiTokens 为 emoji 到占位符的映射表。
// 返回值：返回恢复 emoji 后的 YAML 内容。
func restoreEmojiTokens(content []byte, emojiTokens map[string]string) []byte {
	data := string(content)
	for emojiText, token := range emojiTokens {
		data = string(bytes.ReplaceAll([]byte(data), []byte(token), []byte(emojiText)))
	}
	return []byte(data)
}

// readSubscriptionFile 用于从订阅目录直接读取当前请求对应的原始订阅文件。
// 参数含义：conf 为当前路径配置。
// 返回值：返回订阅文件内容和读取错误。
func (h *SubscribeHandler) readSubscriptionFile(conf config.PathConfig) ([]byte, error) {
	filePath := filepath.Join(h.appConfig.Global.Storage.SubscriptionDir, conf.File)
	stat, err := os.Stat(filePath)
	if err != nil {
		h.deleteCachedSubscriptionFile(conf.Path)
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if cached, ok := h.getCachedSubscriptionFile(conf.Path, stat.ModTime(), stat.Size()); ok {
		return cached, nil
	}

	res, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	h.setCachedSubscriptionFile(conf.Path, stat.ModTime(), stat.Size(), res)
	return res, nil
}

// getCachedSubscriptionFile 用于按 path 和文件元信息读取仍然有效的订阅缓存。
// 参数含义：path 为路由路径；modTime 为当前文件修改时间；size 为当前文件大小。
// 返回值：返回缓存内容以及是否命中。
func (h *SubscribeHandler) getCachedSubscriptionFile(path string, modTime time.Time, size int64) ([]byte, bool) {
	h.fileMu.RLock()
	defer h.fileMu.RUnlock()

	cached, ok := h.fileCache[path]
	if !ok {
		return nil, false
	}

	// 同时比较修改时间和大小，避免仅靠时间戳时漏掉同秒内更新。
	if !cached.modTime.Equal(modTime) || cached.size != size {
		return nil, false
	}

	return cached.content, true
}

// setCachedSubscriptionFile 用于更新指定 path 的订阅文件缓存。
// 参数含义：path 为路由路径；modTime 为文件修改时间；size 为文件大小；content 为文件内容。
// 返回值：无。
func (h *SubscribeHandler) setCachedSubscriptionFile(path string, modTime time.Time, size int64, content []byte) {
	h.fileMu.Lock()
	defer h.fileMu.Unlock()

	h.fileCache[path] = cachedSubscriptionFile{
		content: content,
		modTime: modTime,
		size:    size,
	}
}

// deleteCachedSubscriptionFile 用于在文件不存在或读取失败时清理过期缓存。
// 参数含义：path 为路由路径。
// 返回值：无。
func (h *SubscribeHandler) deleteCachedSubscriptionFile(path string) {
	h.fileMu.Lock()
	defer h.fileMu.Unlock()

	delete(h.fileCache, path)
}
