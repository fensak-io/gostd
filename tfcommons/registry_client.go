package tfcommons

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/go-resty/resty/v2"
	getter "github.com/hashicorp/go-getter"
	tfaddr "github.com/hashicorp/terraform-registry-address"
)

const (
	terraformSDPath = "/.well-known/terraform.json"
	modulesV1Key    = "modules.v1"
)

var (
	tfSrcHdrKey = http.CanonicalHeaderKey("X-Terraform-Get")
)

// RegistryClient is a thin API client for interacting with the Terraform registry protocol.
type RegistryClient struct {
	ModulesEndpoint *url.URL
	// NOTE: Login is currently not supported
	// NOTE: Providers is currently not supported

	httpc *resty.Client
}

func NewRegistryClient(host string) (*RegistryClient, error) {
	httpc := resty.New()
	// TODO: handle authentication by loading terraformrc

	var sdResp ServiceDiscoveryResp
	sdURL := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   terraformSDPath,
	}
	if _, err := httpc.R().SetResult(&sdResp).Get(sdURL.String()); err != nil {
		return nil, err
	}

	if sdResp.ModulesV1 == nil {
		return nil, fmt.Errorf("no modules.v1 key in services list")
	}
	modURLStr := *sdResp.ModulesV1

	// If the registry returns a path, then reuse host to construct the absolute URL.
	if strings.HasPrefix(modURLStr, "/") {
		modURLStr = fmt.Sprintf("https://%s%s", host, modURLStr)
	}

	modURL, err := url.Parse(modURLStr)
	if err != nil {
		return nil, err
	}

	return &RegistryClient{
		ModulesEndpoint: modURL,
		httpc:           httpc,
	}, nil
}

func (regClt *RegistryClient) GetVersions(module tfaddr.ModulePackage) (*ModuleVersionList, error) {
	versionsURL := *regClt.ModulesEndpoint
	versionsURL.Path = path.Join(versionsURL.Path, module.Namespace, module.Name, module.TargetSystem, "versions")

	var resp ModuleVersionsResp
	if _, err := regClt.httpc.R().SetResult(&resp).Get(versionsURL.String()); err != nil {
		return nil, err
	}
	// For forward compatibility, the registry uses a weird response format for the versions endpoint where it returns a
	// singleton list for the versions object. So here we just automatically pull out the first element to return the
	// user friendly representation.
	return &resp.Modules[0], nil
}

// DownloadToPath uses the terraform module registry protocol to download the given module package to the destination
// directory. Note that this will return an error if the destination directory is not empty.
func (regClt *RegistryClient) DownloadToPath(module tfaddr.ModulePackage, version, destDir string) error {
	downloadURL := *regClt.ModulesEndpoint
	downloadURL.Path = path.Join(downloadURL.Path, module.Namespace, module.Name, module.TargetSystem, version, "download")

	resp, err := regClt.httpc.R().Get(downloadURL.String())
	if err != nil {
		return err
	}
	getURL := resp.Header().Get(tfSrcHdrKey)
	return getter.GetAny(destDir, getURL)
}
