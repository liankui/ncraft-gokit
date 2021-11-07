package nacos

import (
	"github.com/go-kit/kit/log"
	kitsd "github.com/go-kit/kit/sd"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/ncraft-io/ncraft-go/pkg/logs"
	"net/url"
	"strconv"
	"strings"
)

type NacosClient struct {
	client naming_client.INamingClient
	param  *vo.RegisterInstanceParam
	logger log.Logger
}

func NewNacosClient(urls []string, cfg *Config, logger log.Logger) *NacosClient {
	if cfg == nil || len(urls) == 0 {
		return nil
	}
	var sc []constant.ServerConfig
	for _, url := range urls {
		ip := strings.Split(url, ":")[0]
		port, err := strconv.Atoi(strings.Split(url, ":")[1])
		if err != nil {
			panic(err)
		}
		sc = append(sc, *constant.NewServerConfig(ip, uint64(port)))
	}

	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &cfg.ClientConfig,
			ServerConfigs: sc,
		},
	)

	if err != nil {
		panic(err)
	}

	nacosClient := &NacosClient{client: namingClient, logger: logger}

	return nacosClient
}

func (c *NacosClient) Register(urlStr, name string, tags []string) error {
	if !strings.HasPrefix(urlStr, "nacos://") {
		urlStr = "nacos://" + urlStr
	}

	param := vo.RegisterInstanceParam{ServiceName: name}

	//if strings.Contains(urlStr, ":") {
	//	param.Ip = strings.Split(urlStr, ":")[0]
	//	p, err := strconv.Atoi(strings.Split(urlStr, ":")[1])
	//	if err != nil {
	//		return err
	//	}
	//	param.Port = uint64(p)
	//}

	url, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	param.Ip = strings.Split(url.Host, ":")[0]
	port, err := strconv.Atoi(strings.Split(url.Host, ":")[1])
	if err != nil {
		return nil
	}
	param.Port = uint64(port)
	param.Healthy = true
	param.Enable = true
	param.Weight = 10

	success, err := c.client.RegisterInstance(param)
	if err != nil {
		return err
	}
	logs.Infof("RegisterServiceInstance,param:%+v,result:%+v \n\n", param, success)
	c.param = &param
	return nil
}

func (c *NacosClient) Deregister() error {
	// Ephemeral 是否临时实例
	dParam := vo.DeregisterInstanceParam{Ip: c.param.Ip, Port: c.param.Port, ServiceName: c.param.ServiceName}

	success, err := c.client.DeregisterInstance(dParam)
	if err != nil {
		return err
	}
	logs.Infof("DeRegisterServiceInstance,param:%+v,result:%+v \n\n", dParam, success)
	return nil
}

func (c *NacosClient) Instancer(service string) kitsd.Instancer {
	if c == nil {
		return nil
	}
	instancer, err := NewInstancer(c, service, "", []string{}, c.logger)
	if err != nil {
		panic(err)
	}
	return instancer
}

func (c *NacosClient) WatchService(service, groupName string, clusters []string, ch chan struct{}) {
	// Subscribe key=serviceName+groupName+cluster
	// 注意:我们可以在相同的key添加多个SubscribeCallback.
	c.client.Subscribe(&vo.SubscribeParam{
		ServiceName: service,
		GroupName:   groupName, // 默认值DEFAULT_GROUP
		Clusters:    clusters,  // 默认值DEFAULT
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			if err != nil {
				return
			}
			ch <- struct{}{}
		},
	})
}

func (c *NacosClient) GetInstance(service string) ([]string, error) {
	// SelectInstances 只返回满足这些条件的实例列表：healthy=${HealthyOnly},enable=true 和weight>0
	instances, err := c.client.SelectInstances(vo.SelectInstancesParam{ServiceName: service, Clusters: []string{""}, GroupName: "DEFAULT_GROUP", HealthyOnly: true})
	if err != nil {
		return nil, err
	}
	var res []string
	// 192.168.129.251#11332#DEFAULT#DEFAULT_GROUP@@se.v1.Id
	for _, instance := range instances {
		ip := strings.Split(instance.InstanceId, "#")[0]
		port := strings.Split(instance.InstanceId, "#")[1]
		res = append(res, ip+":"+port)
	}
	return res, nil
}

func (c *NacosClient) GetInstanceByGroupClusters(service, groupName string, clusters []string) ([]string, error) {
	instances, err := c.client.SelectInstances(vo.SelectInstancesParam{ServiceName: service, Clusters: clusters, GroupName: groupName, HealthyOnly: true})
	if err != nil {
		return nil, err
	}
	var res []string
	for _, instance := range instances {
		res = append(res, instance.InstanceId)
	}
	return res, nil
}
