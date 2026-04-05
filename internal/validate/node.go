package validate

import (
	"fmt"
	"strings"

	"nodeforge/internal/model"
)

func Node(node model.Node) []error {
	var errs []error

	if node.Type == "" {
		errs = append(errs, fmt.Errorf("type is required"))
	} else {
		switch node.Type {
		case model.ProtocolVMess, model.ProtocolVLESS, model.ProtocolTrojan, model.ProtocolSS:
		default:
			errs = append(errs, fmt.Errorf("unsupported type %q", node.Type))
		}
	}

	if strings.TrimSpace(node.Server) == "" {
		errs = append(errs, fmt.Errorf("server is required"))
	}
	if node.Port < 1 || node.Port > 65535 {
		errs = append(errs, fmt.Errorf("port must be between 1 and 65535"))
	}

	switch node.Type {
	case model.ProtocolVMess, model.ProtocolVLESS:
		if strings.TrimSpace(node.UUID) == "" {
			errs = append(errs, fmt.Errorf("uuid is required for %s", node.Type))
		}
	case model.ProtocolTrojan:
		if strings.TrimSpace(node.Password) == "" {
			errs = append(errs, fmt.Errorf("password is required for trojan"))
		}
	case model.ProtocolSS:
		if strings.TrimSpace(node.Password) == "" {
			errs = append(errs, fmt.Errorf("password is required for ss"))
		}
		if strings.TrimSpace(node.Cipher) == "" {
			errs = append(errs, fmt.Errorf("cipher is required for ss"))
		}
	}

	if node.Network == "ws" && node.Path == "" {
		errs = append(errs, fmt.Errorf("path is required when network is ws"))
	}
	if node.Network == "grpc" && node.ServiceName == "" {
		errs = append(errs, fmt.Errorf("service_name is required when network is grpc"))
	}

	return errs
}
