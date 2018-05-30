package appender

import "github.com/kiali/kiali/graph/tree"

// Appender is implemented by any code offering to append a service graph with
// supplemental information.  On error the appender should panic and it will be
// handled as an error response.
type Appender interface {
	// AppendGraph performs the appender work on the provided trees. The list of
	// trees may be initially empty. An appender is allowed to add or remove trees.
	AppendGraph(trees *[]*tree.ServiceNode, namespaceName string)
}

func checkError(err error) {
	if err != nil {
		panic(err.Error)
	}
}
