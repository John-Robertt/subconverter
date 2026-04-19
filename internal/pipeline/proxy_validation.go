package pipeline

import (
	"fmt"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func validateGeneratedProxies(phase string, proxies []model.Proxy) error {
	for _, proxy := range proxies {
		if err := model.ValidateProxyInvariant(proxy); err != nil {
			return &errtype.BuildError{
				Code:    errtype.CodeBuildValidationFailed,
				Phase:   phase,
				Message: fmt.Sprintf("代理 %q 不满足模型不变量：%v", proxy.Name, err),
				Cause:   err,
			}
		}
	}
	return nil
}
