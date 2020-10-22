package business

import (
	"fmt"
	"strings"
)

const (
	destination                = "destination"
	source                     = "source"
	regexGrpcResponseStatusErr = "^[1-9]$|^1[0-6]$"
	regexResponseCodeErr       = "^0$|^[4-5]\\\\d\\\\d$"
)

type LabelsBuilder struct {
	side     string
	peerSide string
	protocol string
	labelsKV []string
}

func NewLabelsBuilder(direction string) *LabelsBuilder {
	side := destination
	peerSide := source
	if direction == "outbound" {
		side = source
		peerSide = destination
	}
	return &LabelsBuilder{
		side:     side,
		peerSide: peerSide,
	}
}

func (lb *LabelsBuilder) Add(key, value string) *LabelsBuilder {
	lb.labelsKV = append(lb.labelsKV, fmt.Sprintf(`%s="%s"`, key, value))
	return lb
}

func (lb *LabelsBuilder) addSided(partialKey, value, side string) *LabelsBuilder {
	lb.labelsKV = append(lb.labelsKV, fmt.Sprintf(`%s_%s="%s"`, side, partialKey, value))
	return lb
}

func (lb *LabelsBuilder) Reporter(name string) *LabelsBuilder {
	return lb.Add("reporter", name)
}

func (lb *LabelsBuilder) SelfReporter() *LabelsBuilder {
	return lb.Add("reporter", lb.side)
}

func (lb *LabelsBuilder) Service(name, namespace string) *LabelsBuilder {
	if lb.side == destination {
		lb.Add("destination_service_name", name)
		if namespace != "" {
			lb.Add("destination_service_namespace", namespace)
		}
	}
	return lb
}

func (lb *LabelsBuilder) Namespace(namespace string) *LabelsBuilder {
	return lb.addSided("workload_namespace", namespace, lb.side)
}

func (lb *LabelsBuilder) Workload(name, namespace string) *LabelsBuilder {
	if namespace != "" {
		lb.addSided("workload_namespace", namespace, lb.side)
	}
	return lb.addSided("workload", name, lb.side)
}

func (lb *LabelsBuilder) App(name, namespace string) *LabelsBuilder {
	if namespace != "" {
		// workload_namespace works for app as well
		lb.addSided("workload_namespace", namespace, lb.side)
	}
	return lb.addSided("canonical_service", name, lb.side)
}

func (lb *LabelsBuilder) PeerService(name, namespace string) *LabelsBuilder {
	if lb.peerSide == destination {
		lb.Add("destination_service_name", name)
		if namespace != "" {
			lb.Add("destination_service_namespace", namespace)
		}
	}
	return lb
}

func (lb *LabelsBuilder) PeerNamespace(namespace string) *LabelsBuilder {
	return lb.addSided("workload_namespace", namespace, lb.peerSide)
}

func (lb *LabelsBuilder) PeerWorkload(name, namespace string) *LabelsBuilder {
	if namespace != "" {
		lb.addSided("workload_namespace", namespace, lb.peerSide)
	}
	return lb.addSided("workload", name, lb.peerSide)
}

func (lb *LabelsBuilder) PeerApp(name, namespace string) *LabelsBuilder {
	if namespace != "" {
		// workload_namespace works for app as well
		lb.addSided("workload_namespace", namespace, lb.peerSide)
	}
	return lb.addSided("canonical_service", name, lb.peerSide)
}

func (lb *LabelsBuilder) Protocol(name string) *LabelsBuilder {
	lb.protocol = strings.ToLower(name)
	return lb.Add("request_protocol", name)
}

func (lb *LabelsBuilder) Aggregate(key, value string) *LabelsBuilder {
	return lb.Add(key, value)
}

func (lb *LabelsBuilder) Build() string {
	return "{" + strings.Join(lb.labelsKV, ",") + "}"
}

func (lb *LabelsBuilder) BuildForErrors() []string {
	errors := []string{}

	// both http and grpc requests can suffer from no response (response_code=0) or an http error
	// (response_code=4xx,5xx), and so we always perform a query against response_code:
	httpLabels := append(lb.labelsKV, fmt.Sprintf(`response_code=~"%s"`, regexResponseCodeErr))
	errors = append(errors, "{"+strings.Join(httpLabels, ",")+"}")

	// if necessary also look for grpc errors. note that the grpc test intentionally avoids
	// `grpc_response_status!="0"`. We need to be backward compatible and handle the case where
	// grpc_response_status does not exist, or if it is simply unset. In Prometheus, negative tests on a
	// non-existent label match everything, but positive tests match nothing. So, we stay positive.
	// furthermore, make sure we only count grpc errors with successful http status.
	if lb.protocol != "http" {
		grpcLabels := append(lb.labelsKV, fmt.Sprintf(`grpc_response_status=~"%s",response_code!~"%s"`, regexGrpcResponseStatusErr, regexResponseCodeErr))
		errors = append(errors, ("{" + strings.Join(grpcLabels, ",") + "}"))
	}
	return errors
}
