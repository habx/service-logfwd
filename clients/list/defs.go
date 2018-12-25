package list

import (
	"github.com/habx/service-logfwd/clients"
	"github.com/habx/service-logfwd/clients/datadog"
	"github.com/habx/service-logfwd/clients/scalyr"
)

var LIST = []clients.OutputClientDefinition{
	scalyr.OutputClientDefinition(),
	datadog.OutputClientDefinition(),
}
