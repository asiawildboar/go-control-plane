// Copyright 2020 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package example

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	tcp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	xdsserver "github.com/envoyproxy/go-control-plane/pkg/server"
)

const (
	ClusterName  = "example_proxy_cluster"
	ListenerName = "listener_0"
	ListenerPort = 10000
	UpstreamHost = "www.envoyproxy.io"
	UpstreamPort = 80
)

func makeCluster(clusterName string) *cluster.Cluster {
	return &cluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment:       makeEndpoint(clusterName),
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
	}
}

func makeTLSCluster(clusterName string) *cluster.Cluster {
	tlsContext := &tls.UpstreamTlsContext{
		CommonTlsContext: &tls.CommonTlsContext{
			TlsCertificateSdsSecretConfigs: []*tls.SdsSecretConfig{{
				Name: "example-cert",
				SdsConfig: &core.ConfigSource{
					ResourceApiVersion: core.ApiVersion_V3,
					ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
						ApiConfigSource: &core.ApiConfigSource{
							ApiType:             core.ApiConfigSource_DELTA_GRPC,
							TransportApiVersion: core.ApiVersion_V3,
							GrpcServices: []*core.GrpcService{{
								TargetSpecifier: &core.GrpcService_GoogleGrpc_{
									GoogleGrpc: &core.GrpcService_GoogleGrpc{
										TargetUri:  "0.0.0.0:18000",
										StatPrefix: "sds",
									},
								},
							}},
						},
					},
				},
			}},
			ValidationContextType: &tls.CommonTlsContext_ValidationContextSdsSecretConfig{
				ValidationContextSdsSecretConfig: &tls.SdsSecretConfig{
					Name: "ca",
					SdsConfig: &core.ConfigSource{
						ResourceApiVersion: core.ApiVersion_V3,
						ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
							ApiConfigSource: &core.ApiConfigSource{
								ApiType:             core.ApiConfigSource_DELTA_GRPC,
								TransportApiVersion: core.ApiVersion_V3,
								GrpcServices: []*core.GrpcService{{
									TargetSpecifier: &core.GrpcService_GoogleGrpc_{
										GoogleGrpc: &core.GrpcService_GoogleGrpc{
											TargetUri:  "0.0.0.0:18000",
											StatPrefix: "sds",
										},
									},
								}},
							},
						},
					},
				},
			},
		},
	}

	tlsAny, err := anypb.New(tlsContext)
	if err != nil {
		panic(err)
	}

	return &cluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment:       makeEndpoint(clusterName),
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
		TransportSocket: &core.TransportSocket{
			Name: "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{
				TypedConfig: tlsAny,
			},
		},
	}
}

func makeSecretValidationContext() *tls.Secret {
	return &tls.Secret{
		Name: "ca",
		Type: &tls.Secret_ValidationContext{
			ValidationContext: &tls.CertificateValidationContext{
				TrustedCa: &core.DataSource{
					Specifier: &core.DataSource_InlineString{InlineString: "fake-ca"},
				},
			},
		},
	}
}

func makeSecretTlsCertificate() *tls.Secret {
	return &tls.Secret{
		Name: "example-cert",
		Type: &tls.Secret_TlsCertificate{
			TlsCertificate: &tls.TlsCertificate{
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineString{InlineString: "fake-private-key"},
				},
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineString{InlineString: "fake-cert"},
				},
			},
		},
	}
}

func makeEndpoint(clusterName string) *endpoint.ClusterLoadAssignment {
	return &endpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: []*endpoint.LbEndpoint{{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.SocketAddress_TCP,
									Address:  UpstreamHost,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: UpstreamPort,
									},
								},
							},
						},
					},
				},
			}},
		}},
	}
}

func makeTCPListener(listenerName string, clusterName string) *listener.Listener {
	filter := &tcp.TcpProxy{
		ClusterSpecifier: &tcp.TcpProxy_Cluster{
			Cluster: clusterName,
		},
		StatPrefix: "tcp_passthrough",
	}
	filterpb, err := anypb.New(filter)
	if err != nil {
		panic(err)
	}

	return &listener.Listener{
		Name: listenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: ListenerPort,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: "tcp-proxy",
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: filterpb,
				},
			}},
		}},
	}
}

func GenerateSnapshot(debug int) *xdsserver.Snapshot {
	clusterName := ClusterName + "_debug_" + fmt.Sprint(debug)
	// snap, _ := xdsserver.NewSnapshot("1",
	// 	map[xdsserver.Type][]proto.Message{
	// 		xdsserver.ClusterType:  {makeCluster(clusterName)},
	// 		xdsserver.ListenerType: {makeTCPListener(ListenerName, clusterName)},
	// 	},
	// )
	snap, _ := xdsserver.NewSnapshot("1",
		map[xdsserver.Type][]proto.Message{
			xdsserver.ClusterType:  {makeTLSCluster(clusterName)},
			xdsserver.ListenerType: {makeTCPListener(ListenerName, clusterName)},
			xdsserver.SecretType:   {makeSecretValidationContext(), makeSecretTlsCertificate()},
		},
	)
	return snap
}
