// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpcgen

import (
	"strings"
	"sync"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/core/v1alpha3"
	"istio.io/istio/pilot/pkg/networking/util"
)

// TODO(gu0keno0): figure out a better way to implement this cache
var (
	cacheMu sync.Mutex
	cache = make(map[string]*route.RouteConfiguration)
	enableRdsCache = true
)

// BuildHTTPRoutes supports per-VIP routes, as used by GRPC.
// This mode is indicated by using names containing full host:port instead of just port.
// Returns true of the request is of this type.
func (g *GrpcConfigGenerator) BuildHTTPRoutes(node *model.Proxy, push *model.PushContext, routeNames []string) model.Resources {
	resp := model.Resources{}
	for _, routeName := range routeNames {
		if rc := buildHTTPRoute(node, push, routeName); rc != nil {
			resp = append(resp, &discovery.Resource{
				Name:     routeName,
				Resource: util.MessageToAny(rc),
			})
		}
	}
	return resp
}

func buildHTTPRoute(node *model.Proxy, push *model.PushContext, routeName string) *route.RouteConfiguration {
	// TODO use route-style naming instead of cluster naming
	_, _, hostname, port := model.ParseSubsetKey(routeName)
	if hostname == "" || port == 0 {
		log.Warn("Failed to parse ", routeName)
		return nil
	}

	if enableRdsCache {
		// TODO(gu0keno0): implement a real cache, maybe use the xDS cache too.
	    cacheMu.Lock()
	    defer cacheMu.Unlock()

    	if routeConfig, ok := cache[routeName]; ok {
		    return routeConfig
	    }
    }

	virtualHosts, _, _ := v1alpha3.BuildSidecarOutboundVirtualHosts(node, push, routeName, port, nil, &model.DisabledCache{})

	// TODO(gu0keno0): clumsy, should definitely refactor once we make it work, see the comments in GetVirtualHostsForSniffedServicePort().
	routeNameParts := strings.Split(routeName, "|")
	if len(routeNameParts) == 4 {
		virtualHosts = v1alpha3.GetVirtualHostsForSniffedServicePort(virtualHosts, routeNameParts[3] + ":" + routeNameParts[1])
	}

	if enableRdsCache {
	    cache[routeName] = &route.RouteConfiguration{
		    Name:         routeName,
		    VirtualHosts: virtualHosts,
	    }
    }

	// Only generate the required route for grpc. Will need to generate more
	// as GRPC adds more features.
	return &route.RouteConfiguration{
		Name:         routeName,
		VirtualHosts: virtualHosts,
	}
}
