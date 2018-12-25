package datadog

import "github.com/habx/service-logfwd/clients"

type outputClientDefinition struct{}

func (t outputClientDefinition) Name() string {
	return "datadog"
}

func (t outputClientDefinition) Config() clients.Config {
	return NewConfig()
}

func (t outputClientDefinition) Create(ch clients.ClientHandler, config clients.Config) clients.OutputClient {
	return NewClient(ch, config)
}

func OutputClientDefinition() clients.OutputClientDefinition {
	return &outputClientDefinition{}
}
