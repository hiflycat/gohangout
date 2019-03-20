package output

import (
	"encoding/json"

	"github.com/childe/gohangout/value_render"
	"github.com/childe/healer"
	"github.com/golang/glog"
)

type KafkaOutput struct {
	BaseOutput
	config map[interface{}]interface{}

	producer *healer.Producer
	key      value_render.ValueRender
}

func NewKafkaOutput(config map[interface{}]interface{}) *KafkaOutput {
	p := &KafkaOutput{
		BaseOutput: NewBaseOutput(config),
		config:     config,
	}

	producerConfig := healer.DefaultProducerConfig()

	var topic string
	if v, ok := config["topic"]; !ok {
		glog.Fatal("kafka output must have topic setting")
	} else {
		topic = v.(string)
	}

	if v, ok := config["bootstrap.servers"]; !ok {
		glog.Fatal("kafka output must have bootstrap.servers setting")
	} else {
		producerConfig.BootstrapServers = v.(string)
	}

	if v, ok := config["compression.type"]; ok {
		producerConfig.CompressionType = v.(string)
	}
	if v, ok := config["message.max.count"]; ok {
		producerConfig.MessageMaxCount = v.(int)
	}
	if v, ok := config["flush.interval.ms"]; ok {
		producerConfig.FlushIntervalMS = v.(int)
	}
	if v, ok := config["metadata.max.age.ms"]; ok {
		producerConfig.MetadataMaxAgeMS = v.(int)
	}

	p.producer = healer.NewProducer(topic, producerConfig)
	if p.producer == nil {
		glog.Fatal("could not create kafka producer")
	}

	if v, ok := config["key"]; ok {
		p.key = value_render.GetValueRender(v.(string))
	} else {
		p.key = nil
	}

	return p
}

func (outputPlugin *KafkaOutput) Emit(event map[string]interface{}) {
	buf, err := json.Marshal(event)
	if err != nil {
		glog.Errorf("marshal %v error: %s", event, err)
		return
	}
	if outputPlugin.key == nil {
		outputPlugin.producer.AddMessage(nil, buf)
	} else {
		key := []byte(outputPlugin.key.Render(event).(string))
		outputPlugin.producer.AddMessage(key, buf)
	}
}

func (outputPlugin *KafkaOutput) Shutdown() {
	outputPlugin.producer.Close()
}
