package connection_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/cloudfoundry-incubator/garden/client/connection"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
)

var _ = Describe("Connection", func() {
	var (
		connection     Connection
		resourceLimits garden.ResourceLimits
		server         *ghttp.Server
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
	})

	JustBeforeEach(func() {
		connection = New("tcp", server.HTTPTestServer.Listener.Addr().String())
	})

	BeforeEach(func() {
		rlimits := &garden.ResourceLimits{
			As:         proto.Uint64(1),
			Core:       proto.Uint64(2),
			Cpu:        proto.Uint64(4),
			Data:       proto.Uint64(5),
			Fsize:      proto.Uint64(6),
			Locks:      proto.Uint64(7),
			Memlock:    proto.Uint64(8),
			Msgqueue:   proto.Uint64(9),
			Nice:       proto.Uint64(10),
			Nofile:     proto.Uint64(11),
			Nproc:      proto.Uint64(12),
			Rss:        proto.Uint64(13),
			Rtprio:     proto.Uint64(14),
			Sigpending: proto.Uint64(15),
			Stack:      proto.Uint64(16),
		}

		resourceLimits = garden.ResourceLimits{
			As:         rlimits.As,
			Core:       rlimits.Core,
			Cpu:        rlimits.Cpu,
			Data:       rlimits.Data,
			Fsize:      rlimits.Fsize,
			Locks:      rlimits.Locks,
			Memlock:    rlimits.Memlock,
			Msgqueue:   rlimits.Msgqueue,
			Nice:       rlimits.Nice,
			Nofile:     rlimits.Nofile,
			Nproc:      rlimits.Nproc,
			Rss:        rlimits.Rss,
			Rtprio:     rlimits.Rtprio,
			Sigpending: rlimits.Sigpending,
			Stack:      rlimits.Stack,
		}
	})

	Describe("Ping", func() {
		Context("when the response is successful", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(200, marshalProto(&protocol.PingResponse{})),
					),
				)
			})

			It("should return the server's capacity", func() {
				err := connection.Ping()
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("should return an error", func() {
				err := connection.Ping()
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Describe("Getting capacity", func() {
		Context("when the response is successful", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/capacity"),
						ghttp.RespondWith(200, marshalProto(&protocol.CapacityResponse{
							MemoryInBytes: proto.Uint64(1111),
							DiskInBytes:   proto.Uint64(2222),
							MaxContainers: proto.Uint64(42),
						}))))
			})

			It("should return the server's capacity", func() {
				capacity, err := connection.Capacity()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(capacity.MemoryInBytes).Should(BeNumerically("==", 1111))
				Ω(capacity.DiskInBytes).Should(BeNumerically("==", 2222))
				Ω(capacity.MaxContainers).Should(BeNumerically("==", 42))
			})
		})

		Context("when the request fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/capacity"),
						ghttp.RespondWith(500, "")))
			})

			It("should return an error", func() {
				_, err := connection.Capacity()
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Describe("Creating", func() {
		BeforeEach(func() {
			ro := protocol.CreateRequest_BindMount_RO
			rw := protocol.CreateRequest_BindMount_RW
			hostOrigin := protocol.CreateRequest_BindMount_Host
			containerOrigin := protocol.CreateRequest_BindMount_Container

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/containers"),
					verifyProtoBody(&protocol.CreateRequest{
						Handle:     proto.String("some-handle"),
						GraceTime:  proto.Uint32(10),
						Rootfs:     proto.String("some-rootfs-path"),
						Network:    proto.String("some-network"),
						Privileged: proto.Bool(false),
						BindMounts: []*protocol.CreateRequest_BindMount{
							{
								SrcPath: proto.String("/src-a"),
								DstPath: proto.String("/dst-a"),
								Mode:    &ro,
								Origin:  &hostOrigin,
							},
							{
								SrcPath: proto.String("/src-b"),
								DstPath: proto.String("/dst-b"),
								Mode:    &rw,
								Origin:  &containerOrigin,
							},
						},
						Properties: []*protocol.Property{
							{
								Key:   proto.String("foo"),
								Value: proto.String("bar"),
							},
						},
						Env: []*protocol.EnvironmentVariable{
							{
								Key:   proto.String("env1"),
								Value: proto.String("env1Value1"),
							},
						},
					}),
					ghttp.RespondWith(200, marshalProto(&protocol.CreateResponse{
						Handle: proto.String("foohandle"),
					}))))
		})

		It("should create a container", func() {
			handle, err := connection.Create(garden.ContainerSpec{
				Handle:     "some-handle",
				GraceTime:  10 * time.Second,
				RootFSPath: "some-rootfs-path",
				Network:    "some-network",
				BindMounts: []garden.BindMount{
					{
						SrcPath: "/src-a",
						DstPath: "/dst-a",
						Mode:    garden.BindMountModeRO,
						Origin:  garden.BindMountOriginHost,
					},
					{
						SrcPath: "/src-b",
						DstPath: "/dst-b",
						Mode:    garden.BindMountModeRW,
						Origin:  garden.BindMountOriginContainer,
					},
				},
				Properties: map[string]string{
					"foo": "bar",
				},
				Env: []string{"env1=env1Value1"},
			})

			Ω(err).ShouldNot(HaveOccurred())
			Ω(handle).Should(Equal("foohandle"))
		})
	})

	Describe("Destroying", func() {
		Context("when destroying succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/foo"),
						ghttp.RespondWith(200, marshalProto(&protocol.DestroyResponse{}))))
			})

			It("should stop the container", func() {
				err := connection.Destroy("foo")
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when destroying fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/foo"),
						ghttp.RespondWith(423, "some error")))
			})

			It("return an appropriate error with the code and message", func() {
				err := connection.Destroy("foo")
				Ω(err).Should(MatchError(Error{423, "some error"}))
			})
		})
	})

	Describe("Stopping", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/containers/foo/stop"),
					verifyProtoBody(&protocol.StopRequest{
						Handle: proto.String("foo"),
						Kill:   proto.Bool(true),
					}),
					ghttp.RespondWith(200, marshalProto(&protocol.StopResponse{}))))
		})

		It("should stop the container", func() {
			err := connection.Stop("foo", true)
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Describe("Limiting Memory", func() {
		Describe("setting the memory limit", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/memory"),
						verifyProtoBody(&protocol.LimitMemoryRequest{
							Handle:       proto.String("foo"),
							LimitInBytes: proto.Uint64(42),
						}),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitMemoryResponse{
							LimitInBytes: proto.Uint64(40),
						})),
					),
				)
			})

			It("should limit memory", func() {
				newLimits, err := connection.LimitMemory("foo", garden.MemoryLimits{
					LimitInBytes: 42,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits.LimitInBytes).Should(BeNumerically("==", 40))
			})
		})

		Describe("getting the memory limit", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/memory"),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitMemoryResponse{
							LimitInBytes: proto.Uint64(40),
						})),
					),
				)
			})

			It("gets the memory limit", func() {
				currentLimits, err := connection.CurrentMemoryLimits("foo")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(currentLimits.LimitInBytes).Should(BeNumerically("==", 40))
			})
		})
	})

	Describe("Limiting CPU", func() {
		Describe("setting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/cpu"),
						verifyProtoBody(&protocol.LimitCpuRequest{
							Handle:        proto.String("foo"),
							LimitInShares: proto.Uint64(42),
						}),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitCpuResponse{
							LimitInShares: proto.Uint64(40),
						})),
					),
				)
			})

			It("should limit CPU", func() {
				newLimits, err := connection.LimitCPU("foo", garden.CPULimits{
					LimitInShares: 42,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits.LimitInShares).Should(BeNumerically("==", 40))
			})
		})

		Describe("getting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/cpu"),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitCpuResponse{
							LimitInShares: proto.Uint64(40),
						})),
					),
				)
			})

			It("sends a nil cpu limit request", func() {
				limits, err := connection.CurrentCPULimits("foo")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits.LimitInShares).Should(BeNumerically("==", 40))
			})
		})
	})

	Describe("Limiting Bandwidth", func() {
		Describe("setting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/bandwidth"),
						verifyProtoBody(&protocol.LimitBandwidthRequest{
							Handle: proto.String("foo"),
							Rate:   proto.Uint64(42),
							Burst:  proto.Uint64(43),
						}),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitBandwidthResponse{
							Rate:  proto.Uint64(1),
							Burst: proto.Uint64(2),
						})),
					),
				)
			})

			It("should limit Bandwidth", func() {
				newLimits, err := connection.LimitBandwidth("foo", garden.BandwidthLimits{
					RateInBytesPerSecond:      42,
					BurstRateInBytesPerSecond: 43,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits.RateInBytesPerSecond).Should(BeNumerically("==", 1))
				Ω(newLimits.BurstRateInBytesPerSecond).Should(BeNumerically("==", 2))
			})
		})

		Describe("getting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/bandwidth"),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitBandwidthResponse{
							Rate:  proto.Uint64(1),
							Burst: proto.Uint64(2),
						})),
					),
				)
			})

			It("sends a nil bandwidth limit request", func() {
				limits, err := connection.CurrentBandwidthLimits("foo")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits.RateInBytesPerSecond).Should(BeNumerically("==", 1))
				Ω(limits.BurstRateInBytesPerSecond).Should(BeNumerically("==", 2))
			})
		})
	})

	Describe("Limiting Disk", func() {
		Describe("setting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/disk"),
						verifyProtoBody(&protocol.LimitDiskRequest{
							Handle: proto.String("foo"),

							BlockSoft: proto.Uint64(42),
							BlockHard: proto.Uint64(42),

							InodeSoft: proto.Uint64(42),
							InodeHard: proto.Uint64(42),

							ByteSoft: proto.Uint64(42),
							ByteHard: proto.Uint64(42),
						}),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitDiskResponse{
							BlockSoft: proto.Uint64(3),
							BlockHard: proto.Uint64(4),
							InodeSoft: proto.Uint64(7),
							InodeHard: proto.Uint64(8),
							ByteSoft:  proto.Uint64(11),
							ByteHard:  proto.Uint64(12),
						})),
					),
				)
			})

			It("should limit disk", func() {
				newLimits, err := connection.LimitDisk("foo", garden.DiskLimits{
					BlockSoft: 42,
					BlockHard: 42,

					InodeSoft: 42,
					InodeHard: 42,

					ByteSoft: 42,
					ByteHard: 42,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits).Should(Equal(garden.DiskLimits{
					BlockSoft: 3,
					BlockHard: 4,
					InodeSoft: 7,
					InodeHard: 8,
					ByteSoft:  11,
					ByteHard:  12,
				}))
			})
		})

		Describe("getting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/disk"),
						ghttp.RespondWith(200, marshalProto(&protocol.LimitDiskResponse{
							BlockSoft: proto.Uint64(3),
							BlockHard: proto.Uint64(4),
							InodeSoft: proto.Uint64(7),
							InodeHard: proto.Uint64(8),
							ByteSoft:  proto.Uint64(11),
							ByteHard:  proto.Uint64(12),
						})),
					),
				)
			})

			It("sends a nil disk limit request", func() {
				limits, err := connection.CurrentDiskLimits("foo")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits).Should(Equal(garden.DiskLimits{
					BlockSoft: 3,
					BlockHard: 4,
					InodeSoft: 7,
					InodeHard: 8,
					ByteSoft:  11,
					ByteHard:  12,
				}))
			})
		})
	})

	Describe("NetIn", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/containers/foo-handle/net/in"),
					verifyProtoBody(&protocol.NetInRequest{
						Handle:        proto.String("foo-handle"),
						HostPort:      proto.Uint32(8080),
						ContainerPort: proto.Uint32(8081),
					}),
					ghttp.RespondWith(200, marshalProto(&protocol.NetInResponse{
						HostPort:      proto.Uint32(1234),
						ContainerPort: proto.Uint32(1235),
					}))))
		})

		It("should return the allocated ports", func() {
			hostPort, containerPort, err := connection.NetIn("foo-handle", 8080, 8081)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(hostPort).Should(Equal(uint32(1234)))
			Ω(containerPort).Should(Equal(uint32(1235)))
		})
	})

	Describe("NetOut", func() {
		var (
			handle           string
			expectedProtocol protocol.NetOutRequest_Protocol
			expectedNetworks []*protocol.NetOutRequest_IPRange
			expectedPorts    []*protocol.NetOutRequest_PortRange
			expectedICMPs    *protocol.NetOutRequest_ICMPControl
			expectedLog      bool
		)

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", fmt.Sprintf("/containers/%s/net/out", handle)),
					verifyProtoBody(&protocol.NetOutRequest{
						Handle:   proto.String(handle),
						Networks: expectedNetworks,
						Ports:    expectedPorts,
						Protocol: &expectedProtocol,
						Icmps:    expectedICMPs,
						Log:      proto.Bool(expectedLog),
					}),
					ghttp.RespondWith(200, marshalProto(&protocol.NetOutResponse{}))))
		})

		BeforeEach(func() {
			expectedNetworks = nil
			expectedPorts = nil
			expectedICMPs = nil
			expectedLog = false
			expectedProtocol = protocol.NetOutRequest_ALL
		})

		Context("when a zero-value NetOutRule is passed", func() {
			It("should send a nil network, portrange and icmpcontrol, logging false and TCP protocol", func() {
				Ω(connection.NetOut(handle, garden.NetOutRule{})).Should(Succeed())
			})
		})

		Context("when the network is zero-length", func() {
			BeforeEach(func() {
				expectedNetworks = nil
			})

			It("should not send any networks", func() {
				Ω(connection.NetOut(handle, garden.NetOutRule{
					Networks: []garden.IPRange{},
				})).Should(Succeed())
			})
		})

		Context("when Network is not nil", func() {
			BeforeEach(func() {
				expectedNetworks = []*protocol.NetOutRequest_IPRange{
					{
						Start: proto.String("1.2.3.4"),
						End:   proto.String("4.3.2.1"),
					}, {
						Start: proto.String("9.8.7.6"),
						End:   proto.String("6.7.8.9"),
					},
				}
			})

			It("should send the networks IPs as strings", func() {
				Ω(connection.NetOut(handle, garden.NetOutRule{
					Networks: []garden.IPRange{
						{
							Start: net.ParseIP("1.2.3.4"),
							End:   net.ParseIP("4.3.2.1"),
						},
						{
							Start: net.ParseIP("9.8.7.6"),
							End:   net.ParseIP("6.7.8.9"),
						},
					},
				})).Should(Succeed())
			})
		})

		Context("when Protocol is supplied", func() {
			BeforeEach(func() {
				expectedProtocol = protocol.NetOutRequest_ICMP
			})

			It("sends the protocol", func() {
				Ω(connection.NetOut(handle, garden.NetOutRule{
					Protocol: garden.ProtocolICMP,
				})).Should(Succeed())
			})

			Context("but the passed protocol is unknown", func() {
				It("returns an error", func() {
					Ω(connection.NetOut(handle, garden.NetOutRule{
						Protocol: garden.Protocol(44),
					})).Should(MatchError("invalid protocol"))
				})
			})
		})

		Context("when Ports is not nil", func() {
			BeforeEach(func() {
				expectedPorts = []*protocol.NetOutRequest_PortRange{
					{
						Start: proto.Uint32(1),
						End:   proto.Uint32(99),
					},
					{
						Start: proto.Uint32(101),
						End:   proto.Uint32(102),
					},
				}
			})

			It("should send the ports as uint32s", func() {
				Ω(connection.NetOut(handle, garden.NetOutRule{
					Ports: []garden.PortRange{
						{
							Start: 1,
							End:   99,
						},
						{
							Start: 101,
							End:   102,
						},
					},
				})).Should(Succeed())
			})
		})

		Context("when ICMPs is not nil", func() {
			Context("and code is nil", func() {
				BeforeEach(func() {
					expectedICMPs = &protocol.NetOutRequest_ICMPControl{
						Type: proto.Uint32(2),
						Code: nil,
					}
				})

				It("sends the type, and a nil code", func() {
					Ω(connection.NetOut(handle, garden.NetOutRule{
						ICMPs: &garden.ICMPControl{Type: 2},
					})).Should(Succeed())
				})
			})

			Context("and code is not nil", func() {
				BeforeEach(func() {
					expectedICMPs = &protocol.NetOutRequest_ICMPControl{
						Type: proto.Uint32(2),
						Code: proto.Int32(42),
					}
				})

				It("sends the type and code", func() {
					var code garden.ICMPCode = 42
					Ω(connection.NetOut(handle, garden.NetOutRule{
						ICMPs: &garden.ICMPControl{
							Type: 2,
							Code: &code,
						},
					})).Should(Succeed())
				})
			})
		})
	})

	Describe("Listing containers", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers", "foo=bar"),
					ghttp.RespondWith(200, marshalProto(&protocol.ListResponse{
						Handles: []string{"container1", "container2", "container3"},
					}))))
		})

		It("should return the list of containers", func() {
			handles, err := connection.List(map[string]string{"foo": "bar"})

			Ω(err).ShouldNot(HaveOccurred())
			Ω(handles).Should(Equal([]string{"container1", "container2", "container3"}))
		})
	})

	Describe("Getting container info", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers/some-handle/info"),
					ghttp.RespondWith(200, marshalProto(&protocol.InfoResponse{
						State:         proto.String("chilling out"),
						Events:        []string{"maxing", "relaxing all cool"},
						HostIp:        proto.String("host-ip"),
						ContainerIp:   proto.String("container-ip"),
						ContainerPath: proto.String("container-path"), ProcessIds: []uint64{1, 2}, Properties: []*protocol.Property{
							{
								Key:   proto.String("prop-key"),
								Value: proto.String("prop-value"),
							},
						},

						MemoryStat: &protocol.InfoResponse_MemoryStat{
							Cache:                   proto.Uint64(1),
							Rss:                     proto.Uint64(2),
							MappedFile:              proto.Uint64(3),
							Pgpgin:                  proto.Uint64(4),
							Pgpgout:                 proto.Uint64(5),
							Swap:                    proto.Uint64(6),
							Pgfault:                 proto.Uint64(7),
							Pgmajfault:              proto.Uint64(8),
							InactiveAnon:            proto.Uint64(9),
							ActiveAnon:              proto.Uint64(10),
							InactiveFile:            proto.Uint64(11),
							ActiveFile:              proto.Uint64(12),
							Unevictable:             proto.Uint64(13),
							HierarchicalMemoryLimit: proto.Uint64(14),
							HierarchicalMemswLimit:  proto.Uint64(15),
							TotalCache:              proto.Uint64(16),
							TotalRss:                proto.Uint64(17),
							TotalMappedFile:         proto.Uint64(18),
							TotalPgpgin:             proto.Uint64(19),
							TotalPgpgout:            proto.Uint64(20),
							TotalSwap:               proto.Uint64(21),
							TotalPgfault:            proto.Uint64(22),
							TotalPgmajfault:         proto.Uint64(23),
							TotalInactiveAnon:       proto.Uint64(24),
							TotalActiveAnon:         proto.Uint64(25),
							TotalInactiveFile:       proto.Uint64(26),
							TotalActiveFile:         proto.Uint64(27),
							TotalUnevictable:        proto.Uint64(28),
						},

						CpuStat: &protocol.InfoResponse_CpuStat{
							Usage:  proto.Uint64(1),
							User:   proto.Uint64(2),
							System: proto.Uint64(3),
						},

						DiskStat: &protocol.InfoResponse_DiskStat{
							BytesUsed:  proto.Uint64(1),
							InodesUsed: proto.Uint64(2),
						},

						BandwidthStat: &protocol.InfoResponse_BandwidthStat{
							InRate:   proto.Uint64(1),
							InBurst:  proto.Uint64(2),
							OutRate:  proto.Uint64(3),
							OutBurst: proto.Uint64(4),
						},

						MappedPorts: []*protocol.InfoResponse_PortMapping{
							&protocol.InfoResponse_PortMapping{
								HostPort:      proto.Uint32(1234),
								ContainerPort: proto.Uint32(5678),
							},
							&protocol.InfoResponse_PortMapping{
								HostPort:      proto.Uint32(1235),
								ContainerPort: proto.Uint32(5679),
							},
						},
					}))))
		})

		It("should return the container's info", func() {
			info, err := connection.Info("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(info.State).Should(Equal("chilling out"))
			Ω(info.Events).Should(Equal([]string{"maxing", "relaxing all cool"}))
			Ω(info.HostIP).Should(Equal("host-ip"))
			Ω(info.ContainerIP).Should(Equal("container-ip"))
			Ω(info.ContainerPath).Should(Equal("container-path"))
			Ω(info.ProcessIDs).Should(Equal([]uint32{1, 2}))

			Ω(info.Properties).Should(Equal(garden.Properties{
				"prop-key": "prop-value",
			}))

			Ω(info.MemoryStat).Should(Equal(garden.ContainerMemoryStat{
				Cache:                   1,
				Rss:                     2,
				MappedFile:              3,
				Pgpgin:                  4,
				Pgpgout:                 5,
				Swap:                    6,
				Pgfault:                 7,
				Pgmajfault:              8,
				InactiveAnon:            9,
				ActiveAnon:              10,
				InactiveFile:            11,
				ActiveFile:              12,
				Unevictable:             13,
				HierarchicalMemoryLimit: 14,
				HierarchicalMemswLimit:  15,
				TotalCache:              16,
				TotalRss:                17,
				TotalMappedFile:         18,
				TotalPgpgin:             19,
				TotalPgpgout:            20,
				TotalSwap:               21,
				TotalPgfault:            22,
				TotalPgmajfault:         23,
				TotalInactiveAnon:       24,
				TotalActiveAnon:         25,
				TotalInactiveFile:       26,
				TotalActiveFile:         27,
				TotalUnevictable:        28,
			}))

			Ω(info.CPUStat).Should(Equal(garden.ContainerCPUStat{
				Usage:  1,
				User:   2,
				System: 3,
			}))

			Ω(info.DiskStat).Should(Equal(garden.ContainerDiskStat{
				BytesUsed:  1,
				InodesUsed: 2,
			}))

			Ω(info.BandwidthStat).Should(Equal(garden.ContainerBandwidthStat{
				InRate:   1,
				InBurst:  2,
				OutRate:  3,
				OutBurst: 4,
			}))

			Ω(info.MappedPorts).Should(Equal([]garden.PortMapping{
				{HostPort: 1234, ContainerPort: 5678},
				{HostPort: 1235, ContainerPort: 5679},
			}))
		})
	})

	Describe("Streaming in", func() {
		Context("when streaming in succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "destination=%2Fbar"),
						func(w http.ResponseWriter, r *http.Request) {
							body, err := ioutil.ReadAll(r.Body)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(string(body)).Should(Equal("chunk-1chunk-2"))
						},
					),
				)
			})

			It("tells garden.to stream, and then streams the content as a series of chunks", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")

				err := connection.StreamIn("foo-handle", "/bar", buffer)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when streaming in returns an error response", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "destination=%2Fbar"),
						ghttp.RespondWith(http.StatusInternalServerError, "no."),
					),
				)
			})

			It("returns an error on close", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")
				err := connection.StreamIn("foo-handle", "/bar", buffer)
				Ω(err).Should(HaveOccurred())

				Ω(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when streaming in fails hard", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "destination=%2Fbar"),
						ghttp.RespondWith(http.StatusInternalServerError, "no."),
						func(w http.ResponseWriter, r *http.Request) {
							server.CloseClientConnections()
						},
					),
				)
			})

			It("returns an error on close", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")

				err := connection.StreamIn("foo-handle", "/bar", buffer)
				Ω(err).Should(HaveOccurred())

				Ω(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})
	})

	Describe("Streaming Out", func() {
		Context("when streaming succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/files", "source=%2Fbar"),
						ghttp.RespondWith(200, "hello-world!"),
					),
				)
			})

			It("asks garden.for the given file, then reads its content", func() {
				reader, err := connection.StreamOut("foo-handle", "/bar")
				Ω(err).ShouldNot(HaveOccurred())

				readBytes, err := ioutil.ReadAll(reader)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(readBytes).Should(Equal([]byte("hello-world!")))

				reader.Close()
			})
		})

		Context("when streaming fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/files", "source=%2Fbar"),
						func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Content-Length", "500")
						},
					),
				)
			})

			It("asks garden.for the given file, then reads its content", func() {
				reader, err := connection.StreamOut("foo-handle", "/bar")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = ioutil.ReadAll(reader)
				reader.Close()
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Describe("Running", func() {
		stdin := protocol.ProcessPayload_stdin
		stdout := protocol.ProcessPayload_stdout
		stderr := protocol.ProcessPayload_stderr

		Context("when streaming succeeds to completion", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.VerifyJSONRepresenting(&protocol.RunRequest{
							Handle:     proto.String("foo-handle"),
							Path:       proto.String("lol"),
							Args:       []string{"arg1", "arg2"},
							Dir:        proto.String("/some/dir"),
							Privileged: proto.Bool(true),
							User:       proto.String(""),
							Rlimits: &protocol.ResourceLimits{
								As:         proto.Uint64(1),
								Core:       proto.Uint64(2),
								Cpu:        proto.Uint64(4),
								Data:       proto.Uint64(5),
								Fsize:      proto.Uint64(6),
								Locks:      proto.Uint64(7),
								Memlock:    proto.Uint64(8),
								Msgqueue:   proto.Uint64(9),
								Nice:       proto.Uint64(10),
								Nofile:     proto.Uint64(11),
								Nproc:      proto.Uint64(12),
								Rss:        proto.Uint64(13),
								Rtprio:     proto.Uint64(14),
								Sigpending: proto.Uint64(15),
								Stack:      proto.Uint64(16),
							},
						}),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42)})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("stdout data")})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("stderr data")})

							var payload protocol.ProcessPayload
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Source:    &stdin,
								Data:      proto.String("stdin data"),
							}))

							transport.WriteMessage(conn, &protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Source:    &stdout,
								Data:      proto.String("roundtripped " + payload.GetData()),
							})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)})
						},
					),
				)
			})

			It("streams the data, closes the destinations, and notifies of exit", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Run("foo-handle", garden.ProcessSpec{
					Path:       "lol",
					Args:       []string{"arg1", "arg2"},
					Dir:        "/some/dir",
					Privileged: true,
					Limits:     resourceLimits,
				}, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				Eventually(stdout).Should(gbytes.Say("stdout data"))
				Eventually(stdout).Should(gbytes.Say("roundtripped stdin data"))
				Eventually(stderr).Should(gbytes.Say("stderr data"))

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the process is terminated", func() {
			termSignal := protocol.ProcessPayload_terminate

			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42)})

							var payload protocol.ProcessPayload
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Signal:    &termSignal,
							}))

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)})
						},
					),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run("foo-handle", garden.ProcessSpec{}, garden.ProcessIO{})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				err = process.Signal(garden.SignalTerminate)
				Ω(err).ShouldNot(HaveOccurred())

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the process is killed", func() {
			killSignal := protocol.ProcessPayload_kill

			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42)})

							var payload protocol.ProcessPayload
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Signal:    &killSignal,
							}))

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)})
						},
					),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run("foo-handle", garden.ProcessSpec{}, garden.ProcessIO{})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				err = process.Signal(garden.SignalKill)
				Ω(err).ShouldNot(HaveOccurred())

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the process's window is resized", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.VerifyJSONRepresenting(&protocol.RunRequest{
							Handle:     proto.String("foo-handle"),
							Path:       proto.String("lol"),
							Args:       []string{"arg1", "arg2"},
							Privileged: proto.Bool(false),
							User:       proto.String(""),
							Tty: &protocol.TTY{
								WindowSize: &protocol.TTY_WindowSize{
									Columns: proto.Uint32(100),
									Rows:    proto.Uint32(200),
								},
							},
							Rlimits: &protocol.ResourceLimits{},
						}),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42)})

							var payload protocol.ProcessPayload
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Tty: &protocol.TTY{
									WindowSize: &protocol.TTY_WindowSize{
										Columns: proto.Uint32(80),
										Rows:    proto.Uint32(24),
									},
								},
							}))

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)})
						},
					),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run("foo-handle", garden.ProcessSpec{
					Path: "lol",
					Args: []string{"arg1", "arg2"},
					TTY: &garden.TTYSpec{
						WindowSize: &garden.WindowSize{
							Columns: 100,
							Rows:    200,
						},
					},
				}, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: gbytes.NewBuffer(),
					Stderr: gbytes.NewBuffer(),
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				err = process.SetTTY(garden.TTYSpec{
					WindowSize: &garden.WindowSize{
						Columns: 80,
						Rows:    24,
					},
				})
				Ω(err).ShouldNot(HaveOccurred())

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the connection breaks before an exit status is received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42)})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("stdout data")})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("stderr data")})
						},
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Run("foo-handle", garden.ProcessSpec{
						Path: "lol",
						Args: []string{"arg1", "arg2"},
						Dir:  "/some/dir",
					}, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Context("when the connection returns an error payload", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.RespondWith(200, marshalProto(
							&protocol.ProcessPayload{ProcessId: proto.Uint32(42)},
							&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("stdout data")},
							&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("stderr data")},
							&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Error: proto.String("oh no!")})),
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Run("foo-handle", garden.ProcessSpec{
						Path: "lol",
						Args: []string{"arg1", "arg2"},
						Dir:  "/some/dir",
					}, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(ContainSubstring("oh no!"))
				})
			})
		})
	})

	Describe("Attaching", func() {
		stdin := protocol.ProcessPayload_stdin
		stdout := protocol.ProcessPayload_stdout
		stderr := protocol.ProcessPayload_stderr

		Context("when streaming succeeds to completion", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("stdout data")})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("stderr data")})

							var payload protocol.ProcessPayload
							err = json.NewDecoder(br).Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Source:    &stdin,
								Data:      proto.String("stdin data"),
							}))

							transport.WriteMessage(conn, &protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Source:    &stdout,
								Data:      proto.String("roundtripped " + payload.GetData()),
							})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)})
						},
					),
				)
			})

			It("should stream", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Attach("foo-handle", 42, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				Eventually(stdout).Should(gbytes.Say("stdout data"))
				Eventually(stderr).Should(gbytes.Say("stderr data"))
				Eventually(stdout).Should(gbytes.Say("roundtripped stdin data"))

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when an error occurs while reading the given stdin stream", func() {
			It("does not send an EOF to close the process's stdin", func() {
				finishedReq := make(chan struct{})

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()
							decoder := json.NewDecoder(br)

							var payload protocol.ProcessPayload
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(protocol.ProcessPayload{
								ProcessId: proto.Uint32(42),
								Source:    &stdin,
								Data:      proto.String("stdin data"),
							}))

							var payload2 protocol.ProcessPayload
							err = decoder.Decode(&payload2)
							Ω(err).Should(HaveOccurred())

							close(finishedReq)
						},
					),
				)

				stdinR, stdinW := io.Pipe()

				_, err := connection.Attach("foo-handle", 42, garden.ProcessIO{
					Stdin: stdinR,
				})
				Ω(err).ShouldNot(HaveOccurred())

				stdinW.Write([]byte("stdin data"))
				stdinW.CloseWithError(errors.New("connection broke"))

				Eventually(finishedReq).Should(BeClosed())
			})
		})

		Context("when the connection returns an error payload", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						ghttp.RespondWith(200, marshalProto(
							&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("stdout data")},
							&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("stderr data")},
							&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Error: proto.String("oh no!")})),
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Attach("foo-handle", 42, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())
					Ω(process.ID()).Should(Equal(uint32(42)))

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(ContainSubstring("oh no!"))
				})
			})
		})

		Context("when the connection breaks before an exit status is received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("stdout data")})

							transport.WriteMessage(conn, &protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("stderr data")})
						},
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Attach("foo-handle", 42, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())
					Ω(process.ID()).Should(Equal(uint32(42)))

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
				})
			})
		})
	})
})

func verifyProtoBody(expectedBodyMessages ...proto.Message) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		defer GinkgoRecover()

		decoder := json.NewDecoder(req.Body)

		for _, msg := range expectedBodyMessages {
			received := protocol.RequestMessageForType(protocol.TypeForMessage(msg))

			err := decoder.Decode(received)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(received).Should(Equal(msg))
		}
	}
}

func marshalProto(messages ...proto.Message) string {
	result := new(bytes.Buffer)
	for _, msg := range messages {
		err := transport.WriteMessage(result, msg)
		Ω(err).ShouldNot(HaveOccurred())
	}

	return result.String()
}
