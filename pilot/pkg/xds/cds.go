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

package xds

import (
	"istio.io/istio/pilot/pkg/features"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/config/schema/kind"
)

type CdsGenerator struct {
	Server *DiscoveryServer
}

var _ model.XdsDeltaResourceGenerator = &CdsGenerator{}

// Map of all configs that do not impact CDS
var skippedCdsConfigs = map[kind.Kind]struct{}{
	kind.Gateway:               {},
	kind.WorkloadEntry:         {},
	kind.WorkloadGroup:         {},
	kind.AuthorizationPolicy:   {},
	kind.RequestAuthentication: {},
	kind.Secret:                {},
	kind.Telemetry:             {},
	kind.WasmPlugin:            {},
	kind.ProxyConfig:           {},
}

// Map all configs that impact CDS for gateways when `PILOT_FILTER_GATEWAY_CLUSTER_CONFIG = true`.
var pushCdsGatewayConfig = map[kind.Kind]struct{}{
	kind.VirtualService: {},
	kind.Gateway:        {},
}

func cdsNeedsPush(req *model.PushRequest, proxy *model.Proxy) bool {
	if req == nil {
		return true
	}
	if !req.Full {
		// CDS only handles full push
		return false
	}
	// If none set, we will always push
	if len(req.ConfigsUpdated) == 0 {
		return true
	}
	for config := range req.ConfigsUpdated {
		if features.FilterGatewayClusterConfig {
			if proxy.Type == model.Router {
				if _, f := pushCdsGatewayConfig[config.Kind]; f {
					return true
				}
			}
		}

		if _, f := skippedCdsConfigs[config.Kind]; !f {
			return true
		}
	}
	return false
}

func (c CdsGenerator) Generate(proxy *model.Proxy, w *model.WatchedResource, req *model.PushRequest) (model.Resources, model.XdsLogDetails, error) {
	log.Debugf("CdsGenerator.Generate: building clusters for %v, watched clusters: %v", proxy.ID, w.ResourceNames)
	if !cdsNeedsPush(req, proxy) {
		return nil, model.DefaultXdsLogDetails, nil
	}
	clusters, logs := c.Server.ConfigGenerator.BuildClusters(proxy, req)

	// TODO(gu0keno0): use map to speed this up.
	clusters_filtered := model.Resources{}
	for _, c := range clusters {
		if len(w.ResourceNames) == 0 {
			clusters_filtered = append(clusters_filtered, c)
			continue
		}
		for _, wrn := range w.ResourceNames {
			if wrn == c.Name {
				clusters_filtered = append(clusters_filtered, c)
			}
		}
	}

	return clusters_filtered, logs, nil
}

// GenerateDeltas for CDS currently only builds deltas when services change. todo implement changes for DestinationRule, etc
func (c CdsGenerator) GenerateDeltas(proxy *model.Proxy, req *model.PushRequest,
	w *model.WatchedResource,
) (model.Resources, model.DeletedResources, model.XdsLogDetails, bool, error) {
	log.Debugf("CdsGenerator.GenerateDeltas: building clusters for %v, watched clusters: %v", proxy.ID, w.ResourceNames)
	if !cdsNeedsPush(req, proxy) {
		return nil, nil, model.DefaultXdsLogDetails, false, nil
	}
	updatedClusters, removedClusters, logs, usedDelta := c.Server.ConfigGenerator.BuildDeltaClusters(proxy, req, w)

	if usedDelta {
		return updatedClusters, removedClusters, logs, true, nil
	} else {
		// TODO(gu0keno0): use map to speed this up.
		clusters_filtered := model.Resources{}
		for _, c := range updatedClusters {
			if len(w.ResourceNames) == 0 {
				clusters_filtered = append(clusters_filtered, c)
				continue
			}
			for _, wrn := range w.ResourceNames {
				if wrn == c.Name || wrn == "*" {
					clusters_filtered = append(clusters_filtered, c)
				}
			}
		}
		return clusters_filtered, removedClusters, logs, false, nil
	}
}
