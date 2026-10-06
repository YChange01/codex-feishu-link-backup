package branding

import (
	_ "embed"
	"encoding/base64"
)

const LogoSVGPath = "/branding/codex-feishu-link-logo.svg"

// LegacyLogoSVGPath keeps previously loaded UI bundles able to request the logo.
const LegacyLogoSVGPath = "/branding/codex-feishu-relay-logo.svg"

//go:embed codex_feishu_link_logo.svg
var logoSVG []byte

var logoSVGDataURI = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(logoSVG)

func LogoSVG() []byte {
	return logoSVG
}

func LogoSVGDataURI() string {
	return logoSVGDataURI
}
