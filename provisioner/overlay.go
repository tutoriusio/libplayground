package provisioner

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	dtypes "docker.io/go-docker/api/types"
	"github.com/tutoriusio/libplayground/config"
	"github.com/tutoriusio/libplayground/docker"
	"github.com/tutoriusio/libplayground/pwd/types"
)

type overlaySessionProvisioner struct {
	dockerFactory docker.FactoryApi
}

func NewOverlaySessionProvisioner(df docker.FactoryApi) SessionProvisionerApi {
	return &overlaySessionProvisioner{dockerFactory: df}
}

func (p *overlaySessionProvisioner) SessionNew(ctx context.Context, s *types.Session) error {
	dockerClient, err := p.dockerFactory.GetForSession(s)
	if err != nil {
		// We assume we are out of capacity
		return fmt.Errorf("Out of capacity")
	}
	u, _ := url.Parse(dockerClient.GetDaemonHost())
	if u.Host == "" {
		s.Host = "localhost"
	} else {
		chunks := strings.Split(u.Host, ":")
		s.Host = chunks[0]
	}

	opts := dtypes.NetworkCreate{Driver: "overlay", Attachable: true}
	if err := dockerClient.CreateNetwork(s.Id, opts); err != nil {
		log.Println("ERROR NETWORKING", err)
		return err
	}
	log.Printf("Network [%s] created for session [%s]\n", s.Id, s.Id)

	ip, err := dockerClient.ConnectNetwork(config.L2ContainerName, s.Id, s.PwdIpAddress)
	if err != nil {
		log.Println(err)
		return err
	}
	s.PwdIpAddress = ip
	log.Printf("Connected %s to network [%s]\n", config.PWDContainerName, s.Id)
	return nil
}
func (p *overlaySessionProvisioner) SessionClose(s *types.Session) error {
	// Disconnect L2 router from the network
	dockerClient, err := p.dockerFactory.GetForSession(s)
	if err != nil {
		log.Println(err)
		return err
	}
	if err := dockerClient.DisconnectNetwork(config.L2ContainerName, s.Id); err != nil {
		if !strings.Contains(err.Error(), "is not connected to the network") {
			log.Println("ERROR NETWORKING", err)
			return err
		}
	}
	log.Printf("Disconnected l2 from network [%s]\n", s.Id)
	if err := dockerClient.DeleteNetwork(s.Id); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			log.Println(err)
			return err
		}
	}

	return nil
}
