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
					Specifier: &core.DataSource_InlineString{
						InlineString: `-----BEGIN CERTIFICATE-----
MIIDSDCCAjACCQDqlMu+JcDpOjANBgkqhkiG9w0BAQsFADBmMQswCQYDVQQGEwJV
UzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzEU
MBIGA1UECgwLRXhhbXBsZSBPcmcxFDASBgNVBAMMC2V4YW1wbGUuY29tMB4XDTI1
MDUxODEyMDAyMFoXDTI2MDUxODEyMDAyMFowZjELMAkGA1UEBhMCVVMxEzARBgNV
BAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xFDASBgNVBAoM
C0V4YW1wbGUgT3JnMRQwEgYDVQQDDAtleGFtcGxlLmNvbTCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBAL6tjAr6FaxYYyLuvTEefrFOpkEYTf0kzPN4e80A
aO322XDp9Z6koTIOsz62/aBSZr+99Zbvsy4wZy0mB/89B+5Ja8NV70IkbK/00vls
fghXXuJwwSVF5syWdJs8cxPrR3QzqMvSNvD37CobDAYuJXSX4rey8CIXjflInvKD
HEpbO5JHutz71A6zhxpYnWmum1Esjsq/ncxSAIgD6MqnmT+pHcLa93qDwERmvZKh
RuxeYld/2oaRtAaMN86IT5i0tlH9YyaJCrVANss94GvJVHJgggdcNC9DiM9djfIJ
VRVWegAV7H3J+tWEwsHPx+mg8fxodA95VGVMcThAFuaUrzcCAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEAafWj9kOyDVkKUpqv2pcfQu4tfLpOQI37XY7+AAp1QgvHwStC
kCkasI8qPEV2xq0nJ7j8ChDAJ+rWETDG4IPVtSfqg1IAcOOQKqoBZAd0kdaJOqIQ
wNJIbUegBpGQs9/JIiqwysy94Ahwf6aF2MI6v2c48k3Kicy0cbmlThhTeixoS0lV
/kQxCZfcjbvFLLIc4WD4pZblbt9jSpfUGKTV5cj6E3yfgTG28Ef+tfCw3m2QnFxi
QCUN0iXot/1mFyawHnnvkM5GFAAYkc9o7wyFyvKjGunmGnJjs8jUk7q6vLJvUdxQ
if3AmbRGeNwtC7WOtpOMR5A2S7j7Y+4e+hkQWg==
-----END CERTIFICATE-----`,
					},
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
					Specifier: &core.DataSource_InlineString{
						InlineString: `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC+rYwK+hWsWGMi
7r0xHn6xTqZBGE39JMzzeHvNAGjt9tlw6fWepKEyDrM+tv2gUma/vfWW77MuMGct
Jgf/PQfuSWvDVe9CJGyv9NL5bH4IV17icMElRebMlnSbPHMT60d0M6jL0jbw9+wq
GwwGLiV0l+K3svAiF435SJ7ygxxKWzuSR7rc+9QOs4caWJ1prptRLI7Kv53MUgCI
A+jKp5k/qR3C2vd6g8BEZr2SoUbsXmJXf9qGkbQGjDfOiE+YtLZR/WMmiQq1QDbL
PeBryVRyYIIHXDQvQ4jPXY3yCVUVVnoAFex9yfrVhMLBz8fpoPH8aHQPeVRlTHE4
QBbmlK83AgMBAAECggEBAKhUjU0jef6sCNjN6jdytGXTCPJugmr4EfbeZmyT8A4j
3dHQuQVUUPngAF1dLopaNFsRV73n3kbodC1nZafuORIjvv6y3oWFom2ztIx9OsYi
W6GL6Pb+vsHeERL6Sp1LF8l90YYeDmKse9CwD+1kz6weagfB8Dwojy2C7s8o79Cz
MG9uVI9YuxaKog3P+2wTBe0A74xfMxLMQCBsOnyxbxXAdiQA9sSQ2isU+CpAIy1n
bh9ZaRts2FH9sVu9vv8J5u5DVd5S3830Z/8bQuJ5tQ58Zy/agKGVsAfVQehT9zBC
4zTAsKpxEXbrUYTTt+I1fCRod2KwHp56W1TNEJGTIAECgYEA++jnr6mxS2LxLiqV
igdEiRQnYmHpliDrphn8aujnJwIJh5q6kKrUxUt3zc3miGytISwEu8zNF+pKMX2T
naBj/gOwdJu9wDcy4FUs/cNzk9437OVW0rtDIUhnTV3qMKJaw1KZgp770mg1NOp3
aBtQDvO1ST7egDzo/cXZm6n3+rkCgYEAwcYfvi8gSwKsAixs8kTmKyiZuD6Te9Zy
ck2MwNDZdMHxqGcIftuLKbdPEFaar0nXFGWALhEs4F0Ck4NwC8EOANYK7mSqu58Y
bSvbWr0Fyiu6L455RbfeD5vk71CgazoW0DOOp7J6ot6ZbaDPCy2MgThlc4tVKMQH
ueSg732WQW8CgYEAjKfKHcJhVVeElSN/5dcTBHs1VnCXTZVKHq+pykQLNTOlAIt7
mmVYcmUmGsrZ6tjLfpcmeXnsFmtiS+nzL3MsAdwrfaCsPZRUmv/UJEkq0qikj2iq
pvWakQ3taDyFE+zDQwZu4olE0IIRG1/DlmSRuheH5MLu16mq6m+7hnhMFzkCgYBx
jKwlQnBmBFbPn0DoZz+Jou0Rbnn2Y6AFIzSL+Na0+MGnsVjlHbna5DRMmrNibJ7A
sQn/9MibYWWVE7yg5qxSCRu2vv7dm0kxEDYmYgX2htE/9PlTxX83Hl91bYXTz+J2
dv/tfUUoE9FM0KMDJdnkDyxEHS32CYmNgVBdhvZ5uQKBgGY5cMuhtb7ToFwNkDE2
P6oeTUuOs+lK59iJLc7exM05+8RQt5CcmkHv5nZfzq4q7dANABy+QZYw2OjAwdBB
7IM847YlKgMJrfBnUru0ilx/B/HBiGKfOSdnhwvUKBhvE8tpcso6qe/5bM8c2/Kv
bJegbkzHJMEvlf8tNAAdWnLp
-----END PRIVATE KEY-----`,
					},
				},
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: `-----BEGIN CERTIFICATE-----
MIIDSDCCAjACCQDqlMu+JcDpOjANBgkqhkiG9w0BAQsFADBmMQswCQYDVQQGEwJV
UzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzEU
MBIGA1UECgwLRXhhbXBsZSBPcmcxFDASBgNVBAMMC2V4YW1wbGUuY29tMB4XDTI1
MDUxODEyMDAyMFoXDTI2MDUxODEyMDAyMFowZjELMAkGA1UEBhMCVVMxEzARBgNV
BAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xFDASBgNVBAoM
C0V4YW1wbGUgT3JnMRQwEgYDVQQDDAtleGFtcGxlLmNvbTCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBAL6tjAr6FaxYYyLuvTEefrFOpkEYTf0kzPN4e80A
aO322XDp9Z6koTIOsz62/aBSZr+99Zbvsy4wZy0mB/89B+5Ja8NV70IkbK/00vls
fghXXuJwwSVF5syWdJs8cxPrR3QzqMvSNvD37CobDAYuJXSX4rey8CIXjflInvKD
HEpbO5JHutz71A6zhxpYnWmum1Esjsq/ncxSAIgD6MqnmT+pHcLa93qDwERmvZKh
RuxeYld/2oaRtAaMN86IT5i0tlH9YyaJCrVANss94GvJVHJgggdcNC9DiM9djfIJ
VRVWegAV7H3J+tWEwsHPx+mg8fxodA95VGVMcThAFuaUrzcCAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEAafWj9kOyDVkKUpqv2pcfQu4tfLpOQI37XY7+AAp1QgvHwStC
kCkasI8qPEV2xq0nJ7j8ChDAJ+rWETDG4IPVtSfqg1IAcOOQKqoBZAd0kdaJOqIQ
wNJIbUegBpGQs9/JIiqwysy94Ahwf6aF2MI6v2c48k3Kicy0cbmlThhTeixoS0lV
/kQxCZfcjbvFLLIc4WD4pZblbt9jSpfUGKTV5cj6E3yfgTG28Ef+tfCw3m2QnFxi
QCUN0iXot/1mFyawHnnvkM5GFAAYkc9o7wyFyvKjGunmGnJjs8jUk7q6vLJvUdxQ
if3AmbRGeNwtC7WOtpOMR5A2S7j7Y+4e+hkQWg==
-----END CERTIFICATE-----`,
					},
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
