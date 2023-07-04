package proxy

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"EH-Proxy/pkg/utils/byteStringConv"
	"strconv"
	"strings"
)

const (
	errWrongNumberArgs = sysPrint.ERROR + "wrong number of arguments"
	errSyntaxErr       = sysPrint.ERROR + "syntax error"
	trueString         = "true"
	falseString        = "false"
)

var (
	ServerExistsReply    = []byte("true")
	ServerNotExistsReply = []byte("false")
	ServerSaveErr        = []byte("Save failed.")
)

// RegisterCommand 注册命令，命令名全小写输入
func (pm *proxyManager) RegisterCommand(cmdName string, cmdFunc func(c *client, args [][]byte) error) {
	if _, exists := pm.commandMap[cmdName]; !exists {
		pm.commandMap[cmdName] = cmdFunc
	}
}

func writeServerInfo(builder *strings.Builder, s *server.Server) {
	builder.WriteString("address: " + s.Addr() + "\n")
	builder.WriteString("weight: " + strconv.FormatInt(int64(s.Weight()), 10) + "\n")
	if s.Probe() != server.NoHealthCheck {
		builder.WriteString("probe: " + s.Probe() + "\n")
		builder.WriteString("last ack timestamp: " + strconv.FormatInt(int64(s.LastAck()), 10) + "\n")
	}
	builder.WriteString("pfail:")
	if s.Pfail() == server.IS_PFAIL {
		builder.WriteString(trueString + "\n")
	} else {
		builder.WriteString(falseString + "\n")
	}
	builder.WriteString("active requests: " + strconv.FormatInt(int64(s.ActiveReq()), 10) + "\n\n")
}

// execInfo info 命令
// 获取 proxy 相关信息
func execInfo(c *client, args [][]byte) error {
	if len(args) != 1 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	p := GetProxyInstance()

	builder := strings.Builder{}
	builder.WriteString("[INFO]\n")
	builder.WriteString("[Proxy]\n")
	builder.WriteString("proxy address: " + p.config.Addr + "\n")
	builder.WriteString("proxy manager address: " + p.config.ManagerAddr + "\n")
	builder.WriteString("circuit breaker option: ")
	if p.config.CircuitBreakerOption {
		builder.WriteString(trueString + "\n")
		builder.WriteString("request timeout: " +
			strconv.FormatInt(p.config.RequestTimeout.Milliseconds(), 10) + "ms\n")
	} else {
		builder.WriteString(falseString + "\n")
	}

	builder.WriteString("health check option: ")
	if p.config.HealthCheckOption {
		builder.WriteString(trueString + "\n")
		builder.WriteString("heahth check interval: " +
			strconv.FormatInt(p.config.HeahthCheckInterval.Milliseconds(), 10) + "ms\n")
		builder.WriteString("pfail time: " +
			strconv.FormatInt(p.config.PfailTime.Milliseconds(), 10) + "ms\n")
	} else {
		builder.WriteString(falseString + "\n")
	}

	builder.WriteString("keep-alive option: ")
	if p.config.KeepAliveOption {
		builder.WriteString(trueString + "\n")
	} else {
		builder.WriteString(falseString + "\n")
	}

	builder.WriteString("load balance type: " + string(p.config.LoadBalancerType) + "\n")
	builder.WriteString("url path check option: ")
	if p.config.UrlPathCheckOption {
		builder.WriteString(trueString + "\n")
		builder.WriteString("url path:\n")
		for path := range p.config.UrlPathMap {
			builder.WriteString("\t- " + path + "\n")
		}
	} else {
		builder.WriteString(falseString + "\n")
	}

	pfailCountStr := strconv.FormatInt(int64(p.serverGroup.PfailCount()), 10)
	builder.WriteString("number of pfail servers: " + pfailCountStr + "\n")

	builder.WriteString("\n[Server]\n")
	idx := 0
	for _, s := range p.serverGroup.serverMap {
		idx++
		builder.WriteString("-----server" + strconv.Itoa(idx) + "-----\n")
		writeServerInfo(&builder, s)
	}

	err := c.Reply(byteStringConv.StringToBytes(builder.String()))
	if err != nil {
		return err
	}
	return nil
}

// execAddServer 添加服务器命令
// 输入格式：AddServer [addr] [weight] [probe]
// 示例：AddServer 127.0.0.1:8080 200 http://127.0.0.1:8081/check/
// addr 填服务器地址，不需要加 scheme（请求时会自动加上 http scheme）
// weight 权重（1~1000000)，可以为空，则会填充默认值 100
// probe 需要带上 http scheme(http://) ，可以为空，则不会对该服务器进行健康检测
func execAddServer(c *client, args [][]byte) error {
	if len(args) < 2 || len(args) > 5 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	addr := byteStringConv.BytesToString(args[1])
	weight := server.DefaultWeight
	probe := server.NoHealthCheck
	var err error
	if len(args) >= 3 {
		weight, err = strconv.Atoi(byteStringConv.BytesToString(args[2]))
		if err != nil {
			err = c.Reply([]byte(errSyntaxErr))
			return err
		}
	}
	if len(args) == 4 {
		probe = byteStringConv.BytesToString(args[3])
	}
	err = GetProxyInstance().serverGroup.AddServer(GetProxyInstance(), addr, int32(weight), probe)
	if err != nil {
		if err == sysPrint.ErrServerExists {
			err = c.Reply([]byte(err.Error()))
			if err != nil {
				return err
			}
		}
		return err
	}
	err = c.Reply(ReplyOK)
	if err != nil {
		return err
	}
	return nil
}

// execDeleteServer 添加服务器命令
// 输入格式：DeleteServer [addr]
// 示例：DeleteServer 127.0.0.1:8080
// addr 填服务器地址，不需要加 scheme
func execDeleteServer(c *client, args [][]byte) error {
	if len(args) != 2 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	addr := byteStringConv.BytesToString(args[1])
	err := GetProxyInstance().serverGroup.DeleteServer(addr)
	if err != nil {
		if err == sysPrint.ErrServerNotExists {
			err = c.Reply([]byte(err.Error()))
			if err != nil {
				return err
			}
		}
		return err
	}
	err = c.Reply(ReplyOK)
	if err != nil {
		return err
	}
	return nil
}

// execExistsServer 查询服务器是否存在命令
// 输入格式：Exists [addr]
// 示例：ExistsServer 127.0.0.1:8080
// addr 填服务器地址，不需要加 scheme
// 服务器存在返回 1，否则返回 0
func execExistsServer(c *client, args [][]byte) error {
	if len(args) != 2 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	addr := byteStringConv.BytesToString(args[1])
	exists := GetProxyInstance().serverGroup.IsServerExists(addr)
	if exists == true {
		err := c.Reply(ServerExistsReply)
		if err != nil {
			return err
		}
	} else {
		err := c.Reply(ServerNotExistsReply)
		if err != nil {
			return err
		}
	}
	return nil
}

// execGetServer 获取指定服务器信息命令
// 输入格式：GetServer [addr]
// 示例：GetServer 127.0.0.1:8080
// addr 填服务器地址，不需要加 scheme
func execGetServer(c *client, args [][]byte) error {
	if len(args) != 2 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	addr := byteStringConv.BytesToString(args[1])
	s, err := GetProxyInstance().serverGroup.GetServer(addr)
	if err != nil {
		if err == sysPrint.ErrServerNotExists {
			err = c.Reply(byteStringConv.StringToBytes(err.Error()))
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		b := strings.Builder{}
		writeServerInfo(&b, s)
		err = c.Reply(byteStringConv.StringToBytes(b.String()))
		if err != nil {
			return err
		}
	}
	return nil
}

// execSetWeight 设置服务器权重
// 输入格式：SetWeight [addr] [weight]
// 示例：SetWeight 127.0.0.1:8080 200
// addr 填服务器地址，不需要加 scheme
// weight 填需要设置的权重
func execSetWeight(c *client, args [][]byte) error {
	if len(args) != 3 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	addr := byteStringConv.BytesToString(args[1])
	weight, err := strconv.Atoi(byteStringConv.BytesToString(args[2]))
	if err != nil {
		err = c.Reply([]byte(errSyntaxErr))
		return err
	}
	err = GetProxyInstance().serverGroup.SetWeight(addr, int32(weight))
	if err != nil {
		if err == sysPrint.ErrServerNotExists {
			err = c.Reply(byteStringConv.StringToBytes(err.Error()))
			if err != nil {
				return err
			}
		}
		return err
	}
	err = c.Reply(ReplyOK)
	if err != nil {
		return err
	}
	return nil
}

// execShutdown 关闭服务器命令
// 输入格式：Shutdown
func execShutdown(c *client, args [][]byte) error {
	if len(args) != 1 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	err := c.Reply(ReplyOK)
	if err != nil {
		return err
	}
	GetProxyInstance().Shutdown()
	GetProxyManagerInstance().Shutdown()
	return nil
}

// execSave 保存服务器当前配置到本地配置文件命令
// 输入格式：Save
func execSave(c *client, args [][]byte) error {
	if len(args) != 1 {
		err := c.Reply([]byte(errWrongNumberArgs))
		return err
	}
	p := GetProxyInstance()
	err := p.serverGroup.saveServerListToDisk()
	if err != nil {
		err = c.Reply(ServerSaveErr)
		return err
	}
	err = c.Reply(ReplyOK)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	pm := GetProxyManagerInstance()
	pm.RegisterCommand("info", execInfo)
	pm.RegisterCommand("addserver", execAddServer)
	pm.RegisterCommand("deleteserver", execDeleteServer)
	pm.RegisterCommand("setweight", execSetWeight)
	pm.RegisterCommand("exists", execExistsServer)
	pm.RegisterCommand("getserver", execGetServer)
	pm.RegisterCommand("shutdown", execShutdown)
	pm.RegisterCommand("save", execSave)
}
