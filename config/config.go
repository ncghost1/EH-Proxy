package config

import (
	"EH-Proxy/pkg/slb"
	"EH-Proxy/pkg/system/sysPrint"
	"EH-Proxy/pkg/utils/datastructure"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const (
	defaultAddr                = "127.0.0.1:5200"
	defaultManagerAddr         = "127.0.0.1:5201"
	defaultConfigFilePath      = "config.yaml"
	defaultCiruitBreakerOption = false
	defaultRequestTimeout      = 3000 * time.Millisecond
	defaultHealthCheckOption   = true
	defaultHealthCheckInterval = 1 * time.Second
	defaultPfailTime           = 3 * time.Second
	defaultUrlPathCheck        = false
	defaultLoadBalancerType    = slb.RoundRobin
	defaultKeepAliveOption     = false
)

var (
	ConfigFilePath string
)

type ProxyConfig struct {
	Addr                 string        `yaml:"proxy-addr"`             // proxy 连接地址
	ManagerAddr          string        `yaml:"proxy-manager-addr"`     // proxy manager 连接地址
	CircuitBreakerOption bool          `yaml:"circuit-breaker-option"` // 熔断机制/断路器开关
	RequestTimeout       time.Duration `yaml:"request-timeout"`        // 请求超时时间（开启断路器后有效）
	HealthCheckOption    bool          `yaml:"health-check-option"`    // 健康检测开关
	HeahthCheckInterval  time.Duration `yaml:"heahth-check-interval"`  // 每次健康检测间隔
	PfailTime            time.Duration `yaml:"pfail-time"`             // 认为下线需要的未响应时间

	// proxy 和 server 之间的连接是否使用 keepAlive，
	// 默认关闭，请确保调整系统与进程最大打开文件数足够大后再在 config.yaml 中设为 true
	KeepAliveOption    bool                 `yaml:"keep-alive-option"`
	LoadBalancerType   slb.LoadBalancerType `yaml:"load-balancer-type"`     // 负载均衡器类型
	UrlPathCheckOption bool                 `yaml:"url-path-check-option"`  // URL 路径匹配开关
	UrlPathMap         map[string]struct{}  `yaml:"url-path-map,omitempty"` // URL 路径完全匹配哈希表
	UrlPathTrie        *datastructure.Trie  `yaml:"-"`                      // URL 路径前缀树，用于前缀匹配
	InitServerList     []ServerConfig       `yaml:"server-list,omitempty"`  // 初始化服务器列表
}

type ServerConfig struct {
	Addr   string `yaml:"addr"`   // 服务器连接地址（IP:PORT）
	Weight int32  `yaml:"weight"` // 权重
	Probe  string `yaml:"probe"`  // 健康监测请求地址，需要加上 HTTP Scheme(http://)
}

func init() {
	ConfigFilePath = defaultConfigFilePath
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-configPath" && i+1 < len(args) {
			ConfigFilePath = args[i+1]
			break
		}
	}
}

func NewProxyConfig() (*ProxyConfig, error) {
	var file *os.File
	if _, err := os.Stat(ConfigFilePath); os.IsNotExist(err) {
		file, err = os.Create(ConfigFilePath)
		defer file.Close()
		if err != nil {
			sysPrint.PrintlnSystemMsg("Failed to create config file: " + err.Error())
			return nil, err
		}
		proxyConfig, err := createDefaultConfig(file)
		if err != nil {
			return nil, err
		}
		return proxyConfig, nil
	} else {
		file, err = os.Open(ConfigFilePath)
		defer file.Close()
		if err != nil {
			sysPrint.PrintlnSystemMsg("Failed to open config file: " + err.Error())
			return nil, err
		}
		var pc *ProxyConfig
		buf, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(buf, &pc)
		if err != nil {
			return nil, err
		}
		for path := range pc.UrlPathMap {
			if strings.HasSuffix(path, "*") {
				pc.UrlPathTrie.Insert(path)
			}
		}
		return pc, nil
	}
}

func createDefaultConfig(file *os.File) (*ProxyConfig, error) {
	pc := &ProxyConfig{
		Addr:                 defaultAddr,
		ManagerAddr:          defaultManagerAddr,
		RequestTimeout:       defaultRequestTimeout,
		CircuitBreakerOption: defaultCiruitBreakerOption,
		HealthCheckOption:    defaultHealthCheckOption,
		HeahthCheckInterval:  defaultHealthCheckInterval,
		PfailTime:            defaultPfailTime,
		UrlPathCheckOption:   defaultUrlPathCheck,
		UrlPathMap:           nil,
		UrlPathTrie:          nil,
		LoadBalancerType:     defaultLoadBalancerType,
		InitServerList:       nil,
		KeepAliveOption:      defaultKeepAliveOption,
	}
	yamlData, err := yaml.Marshal(&pc)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}
	_, err = file.Write(yamlData)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

// WriteConfig 将 proxyConfig 写入本地配置文件
func WriteConfig(pc *ProxyConfig) error {
	file, err := os.OpenFile(ConfigFilePath, os.O_RDWR|os.O_CREATE, 0644)
	yamlData, err := yaml.Marshal(&pc)
	if err != nil {
		return err
	}
	_, err = file.Write(yamlData)
	if err != nil {
		return err
	}
	return nil
}
