// Package stackpin is the typed registry of every exact toolchain and
// dependency pin. The canonical text source is
// docs/00_context/technology_versions.md; Q1 keeps both in lock-step
// (architecture_conformance.md §4 item 12).
package stackpin

// Toolchain pins.
const (
	GoVersion            = "1.27.1"
	GoModuleName         = "thinhthan"
	ProtocVersion        = "36.2"
	ProtocGenGoVersion   = "v1.36.12"
	PostgresVersion      = "18.6"
	PostgresLinuxImage   = "postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722"
	StaticcheckModule    = "honnef.co/go/tools/cmd/staticcheck"
	StaticcheckVersion   = "v0.8.1" // module version; distribution label 2026.2.1
	StaticcheckDistLabel = "2026.2.1"
	UnityEditorVersion   = "6000.6.1f1"
	UnityEditorChangeset = "7efac9f6c10e"
	GcloudVersion        = "586.0.0"
	GitForWindowsVersion = "2.55.0.windows.5"
)

// InstallerPin is a downloadable installer bound by URL + SHA-256.
type InstallerPin struct {
	URL    string
	SHA256 string
}

// GoModulePins is the allowed set of direct Go module requirements with exact
// versions (technology_versions.md § Backend). A go.mod require absent from
// this map, or at a different version, is an unlisted dependency (Q1).
var GoModulePins = map[string]string{
	"github.com/jackc/pgx/v5":                                           "v5.11.0",
	"github.com/coder/websocket":                                        "v1.8.15",
	"github.com/golang-migrate/migrate/v4":                              "v4.20.1",
	"google.golang.org/protobuf":                                        "v1.36.12",
	"go.opentelemetry.io/otel":                                          "v1.46.0",
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp":     "v0.71.0",
	"go.opentelemetry.io/otel/sdk":                                      "v1.46.0",
	"go.opentelemetry.io/otel/sdk/metric":                               "v1.46.0",
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp":   "v1.46.0",
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp": "v1.46.0",
	"golang.org/x/text":                                                 "v0.42.0",
	"golang.org/x/crypto":                                               "v0.57.0",
	"github.com/clipperhouse/uax29/v2":                                  "v2.7.0",
	// Matrix-declared transitive require-closure of the pinned pgx/migrate
	// modules (technology_versions.md § Backend); the exact commit
	// pseudo-versions below are approved pins, not floating versions.
	"github.com/jackc/pgerrcode":     "v0.0.0-20220416144525-469b46aa5efa",
	"github.com/jackc/pgpassfile":    "v1.0.0",
	"github.com/jackc/pgservicefile": "v0.0.0-20240606120523-5a60cdf6a761",
	"github.com/jackc/puddle/v2":     "v2.2.2",
	"golang.org/x/sync":              "v0.23.0",
}

// TransitiveModuleAllowlist records the only non-direct modules the approved
// closure may contain (technology_versions.md § Backend transitive rules).
var TransitiveModuleAllowlist = map[string]string{
	"google.golang.org/grpc":         "v1.83.1", // only via pinned OTel HTTP exporters
	"go.opentelemetry.io/proto/otlp": "v1.11.0", // only via pinned OTel HTTP exporters
}

// UnityPackagePins is the exact com.unity.* dependency set in
// client/Packages/manifest.json (technology_versions.md § Unity).
var UnityPackagePins = map[string]string{
	"com.unity.2d.animation":                       "16.0.0",
	"com.unity.2d.psdimporter":                     "15.0.0",
	"com.unity.addressables":                       "2.11.2",
	"com.unity.inputsystem":                        "1.20.0",
	"com.unity.localization":                       "1.5.12",
	"com.unity.memoryprofiler":                     "1.1.12",
	"com.unity.performance.profile-analyzer":       "1.4.0",
	"com.unity.render-pipelines.universal":         "17.6.0",
	"com.unity.render-pipelines.core":              "17.6.0",
	"com.unity.test-framework":                     "1.6.0",
	"com.unity.test-framework.performance":         "6.6.0",
	"com.unity.ugui":                               "2.0.0",
	"com.unity.modules.animation":                  "1.0.0",
	"com.unity.modules.audio":                      "1.0.0",
	"com.unity.modules.imageconversion":            "1.0.0",
	"com.unity.modules.imgui":                      "1.0.0",
	"com.unity.modules.jsonserialize":              "1.0.0",
	"com.unity.modules.particlesystem":             "1.0.0",
	"com.unity.modules.physics":                    "1.0.0",
	"com.unity.modules.physics2d":                  "1.0.0",
	"com.unity.modules.screencapture":              "1.0.0",
	"com.unity.modules.tilemap":                    "1.0.0",
	"com.unity.modules.ui":                         "1.0.0",
	"com.unity.modules.uielements":                 "1.0.0",
	"com.unity.modules.unitywebrequest":            "1.0.0",
	"com.unity.modules.unitywebrequestassetbundle": "1.0.0",
	"com.unity.modules.unitywebrequestaudio":       "1.0.0",
	"com.unity.modules.unitywebrequesttexture":     "1.0.0",
	"com.unity.modules.unitywebrequestwww":         "1.0.0",
	"com.unity.modules.video":                      "1.0.0",
}

// ForbiddenModules must never appear in the resolved module graph
// (AGENTS.md forbidden list; first-party gRPC imports stay denied regardless).
var ForbiddenModules = []string{
	"github.com/gin-gonic/gin",
	"github.com/go-chi/chi",
	"github.com/labstack/echo",
	"github.com/gofiber/fiber",
	"github.com/gorilla/mux",
	"github.com/gorilla/websocket",
	"gorm.io/gorm",
	"github.com/jmoiron/sqlx",
	"go.uber.org/zap",
	"github.com/sirupsen/logrus",
	"github.com/rs/zerolog",
	"github.com/redis/go-redis",
	"github.com/go-redis/redis",
	"github.com/IBM/sarama",
	"github.com/Shopify/sarama",
	"github.com/segmentio/kafka-go",
	"github.com/nats-io/nats.go",
	"golang.org/x/time",
}

// ForbiddenImportPrefixes are forbidden as first-party imports (Q1/Q4).
var ForbiddenImportPrefixes = []string{
	"github.com/gin-gonic/gin",
	"github.com/go-chi/chi",
	"github.com/labstack/echo",
	"github.com/gofiber/fiber",
	"github.com/gorilla/",
	"gorm.io/",
	"github.com/jmoiron/sqlx",
	"go.uber.org/zap",
	"github.com/sirupsen/logrus",
	"github.com/rs/zerolog",
	"github.com/redis/",
	"github.com/go-redis/",
	"github.com/IBM/sarama",
	"github.com/Shopify/sarama",
	"github.com/segmentio/kafka-go",
	"github.com/nats-io/",
	"google.golang.org/grpc",
	"golang.org/x/time/rate",
}

// ForbiddenImportExact are forbidden only on an exact import-path match
// (math/rand has no subpackages; math/rand/v2 is the approved RNG).
var ForbiddenImportExact = []string{
	"math/rand",
}

// UnityWindowsInstallers are the pinned native Windows editor installers
// (matrix: `UnityWindowsInstallers`).
var UnityWindowsInstallers = map[string]InstallerPin{
	"editor": {
		URL:    "https://download.unity3d.com/download_unity/7efac9f6c10e/Windows64EditorInstaller/UnitySetup64-6000.6.1f1.exe",
		SHA256: "8884daa489c8708c17da571c46a869839dd7bbf45f69048bd4db5c6d054d5e36",
	},
	"il2cpp": {
		URL:    "https://download.unity3d.com/download_unity/7efac9f6c10e/TargetSupportInstaller/UnitySetup-Windows-IL2CPP-Support-for-Editor-6000.6.1f1.exe",
		SHA256: "03f0cadf1e54f3eb80bb59865e95bfd7725c9c3b94d22ed0f175a23ac5e5bde4",
	},
}

// AndroidModules are the pinned Android Support installer and submodules used
// by IMP-067 / IMP-096 player builds on the native Windows editor (ADR-0078).
var AndroidModules = map[string]AssetPin{
	"android-support": {
		URL:    "https://download.unity3d.com/download_unity/7efac9f6c10e/TargetSupportInstaller/UnitySetup-Android-Support-for-Editor-6000.6.1f1.exe",
		SHA256: "7fce3760578959becae2aadf80ecc788fd5cdfd9df0e8d45f60faeb643f97aa3",
		Size:   1324518752,
	},
	"openjdk-17.0.18+8": {
		URL:    "https://download.unity3d.com/download_unity/open-jdk/open-jdk-win-x64/jdk17.0.18-8_15e8817d1f5db6db3571ebe7430ef37f7fa8e60e8ff6f3e18ca1cb4c29f78774.zip",
		SHA256: "15e8817d1f5db6db3571ebe7430ef37f7fa8e60e8ff6f3e18ca1cb4c29f78774",
		Size:   118110508,
	},
	"sdk-tools": {
		URL:    "https://download.unity3d.com/download_unity/android-sdk-tools/1_5312bb398affd0d94b90d3780976e1a162aa91944ef6bb40c8feb10ad6cc360d.zip",
		SHA256: "5312bb398affd0d94b90d3780976e1a162aa91944ef6bb40c8feb10ad6cc360d",
		Size:   166,
	},
	"ndk-r27c": {
		URL:    "https://dl.google.com/android/repository/android-ndk-r27c-windows.zip",
		SHA256: "27e49f11e0cee5800983d8af8f4acd5bf09987aa6f790d4439dda9f3643d2494",
		Size:   781511249,
	},
	"cmake-3.22.1": {
		URL:    "https://dl.google.com/android/repository/cmake-3.22.1-windows.zip",
		SHA256: "c9a9d568452a20cf27d703cb21ef9529dd67bda6be048c4d4b884acb3ac3a2b8",
		Size:   16116742,
	},
	"build-tools-36.0.0": {
		URL:    "https://dl.google.com/android/repository/build-tools_r36_windows.zip",
		SHA256: "aa1095cb14d83e483818a748a2c06faaeb8e601561b06a356a119a1b2ca280d3",
		Size:   58699878,
	},
	"platform-tools-36.0.0": {
		URL:    "https://dl.google.com/android/repository/platform-tools_r36.0.0-win.zip",
		SHA256: "12c2841f354e92a0eb2fd7bf6f0f9bf8538abce7bd6b060ac8349d6f6a61107c",
		Size:   7138784,
	},
	"platform-34": {
		URL:    "https://dl.google.com/android/repository/platform-34-ext7_r02.zip",
		SHA256: "5323311cc3e4ad614f0b8053c72b651726f3422448cedd39e48f00737cda8ad0",
	},
	"platform-36": {
		URL:    "https://dl.google.com/android/repository/platform-36_r02.zip",
		SHA256: "37607369a28c5b640b3a7998868d45898ebcb777565a0e85f9acf36f29631d2e",
	},
	"platform-37.0": {
		URL:    "https://dl.google.com/android/repository/platform-37.0_r02.zip",
		SHA256: "840b23e827f96e64aea4c89a1194aac3dc5f6bad37edb231c5c795d890330e8d",
	},
	"commandlinetools-16.0": {
		URL:    "https://dl.google.com/android/repository/commandlinetools-win-12266719_latest.zip",
		SHA256: "f9088c04a44f1f37a8a3a228a7663e11ae9445fa07529c96cef38acb985a88f3",
		Size:   143481958,
	},
}

// AssetPin is a downloadable blob bound by URL + SHA-256 (+ size when pinned).
type AssetPin struct {
	URL    string
	SHA256 string
	Size   int64
}

// EDBPostgresZip is the pinned Windows PostgreSQL 18.6 binaries (EDB).
var EDBPostgresZip = AssetPin{
	URL:    "https://get.enterprisedb.com/postgresql/postgresql-18.6-1-windows-x64-binaries.zip",
	SHA256: "fbe23da234ee31547bf8a36d29dfd81e82b849df2d2b78d2eecb43d360252f8c",
	Size:   343808005,
}

// GoogleProtobufNupkg is the pinned Google.Protobuf NuGet package whose
// lib/netstandard2.0/Google.Protobuf.dll ships in client/Assets/Plugins.
var GoogleProtobufNupkg = AssetPin{
	URL:    "https://api.nuget.org/v3-flatcontainer/google.protobuf/3.36.2/google.protobuf.3.36.2.nupkg",
	SHA256: "1182590db175f9057707857a1df48b217226d0732716cd353fa4aa4683d38dcb",
}

// ProtocZips are the pinned protoc 36.2 release-asset zips; CI downloads and
// SHA-256-verifies them before any codegen gate may run.
var ProtocZips = map[string]InstallerPin{
	"linux": {
		URL:    "https://github.com/protocolbuffers/protobuf/releases/download/v36.2/protoc-36.2-linux-x86_64.zip",
		SHA256: "121f6c7afe1d4d0e3ea6aab9432038599250134cbf4474cb1167d2c7decd4278",
	},
	"windows": {
		URL:    "https://github.com/protocolbuffers/protobuf/releases/download/v36.2/protoc-36.2-win64.zip",
		SHA256: "f0c128dc0d8492eceece83bb459a4c0e316764b929ffbf1aa416357fd644edd3",
	},
}

// GoogleProtobufNupkgVersion is the package version string (manifest parity).
const GoogleProtobufNupkgVersion = "3.36.2"

// CacertDownloadURL and CacertSHA256 pin the CA bundle used for TLS on agents.
const (
	CacertDownloadURL = "https://curl.se/ca/cacert-2026-08-13.pem"
	CacertSHA256      = "f66dff1bdf8f96060b8177976f8b7d9254bc89bc4db933d769f7384d28480bc9"
)

// CliTool is a pinned CLI tool release asset.
type CliTool struct {
	Name          string
	Version       string
	ReleaseURL    string
	LinuxAsset    string
	WindowsAsset  string
	LinuxSHA256   string
	WindowsSHA256 string
}

// CliTools are the release-asset pins installed on every job (ADR-0068 item 7).
var CliTools = []CliTool{
	{
		Name:          "pwsh",
		ReleaseURL:    "https://github.com/PowerShell/PowerShell/releases/download/v7.6.6/",
		Version:       "7.6.6",
		LinuxAsset:    "powershell-7.6.6-linux-x64.tar.gz",
		WindowsAsset:  "PowerShell-7.6.6-win-x64.zip",
		LinuxSHA256:   "ddbc4a2d113bbd46d283cfedcbcd117a70caefd7673f41f2b4e0000badf103bc",
		WindowsSHA256: "02fe458be20493fbdf43f61ea20610b811ee6c738ab1676c61b9cfcd1a33c860",
	},
	{
		Name:          "gh",
		ReleaseURL:    "https://github.com/cli/cli/releases/download/v2.101.0/",
		Version:       "2.101.0",
		LinuxAsset:    "gh_2.101.0_linux_amd64.tar.gz",
		WindowsAsset:  "gh_2.101.0_windows_amd64.zip",
		LinuxSHA256:   "9bca2d1c16825f109907a23307628a2f0698fbf99662b73a5cf0b020293072b8",
		WindowsSHA256: "bc6c814367b193cd8e713611d61e36013c0ef843b8f516458fe3eda039192794",
	},
	{
		Name:          "jq",
		ReleaseURL:    "https://github.com/jqlang/jq/releases/download/jq-1.8.2/",
		Version:       "1.8.2",
		LinuxAsset:    "jq-linux-amd64",
		WindowsAsset:  "jq-windows-amd64.exe",
		LinuxSHA256:   "b1c22172dd303f3be49e935aa56aa48a8b7a46e0bc838b4997d3bb451495870f",
		WindowsSHA256: "a6fc67fedaf9128a3309a1e2ebb8b986aeccf70122ee46d2cb4849e423f0c627",
	},
	{
		Name:          "git-lfs",
		ReleaseURL:    "https://github.com/git-lfs/git-lfs/releases/download/v3.8.0/",
		Version:       "3.8.0",
		LinuxAsset:    "git-lfs-linux-amd64-v3.8.0.tar.gz",
		WindowsAsset:  "git-lfs-windows-amd64-v3.8.0.zip",
		LinuxSHA256:   "e455e00f15d9b95661b8d53498ffb0c3367962cf1ec73c31ab7369516cd6ab8d",
		WindowsSHA256: "b62e7b8ceddee635f691233d77de8eaa4b213e9209e0173811d8cfa77f7882c1",
	},
}

// Actions are the pinned third-party GitHub Actions (full-length commit SHA).
var Actions = map[string]string{
	"actions/checkout":                "11bd71901bbe5b1630ceea73d27597364c9af683", // v4.2.2
	"actions/setup-go":                "f111f3307d8850f501ac008e886eec1fd1932a34", // v5.3.0
	"actions/upload-artifact":         "ea165f8d65b6e75b540449e92b4886f43607fa02", // v4.6.2
	"actions/download-artifact":       "d3f86a106a0bac45b974a628896c90dbdf5c8093", // v4.3.0
	"actions/cache":                   "55cc8345863c7cc4c66a329aec7e433d2d1c52a9", // v6.1.0
	"actions/create-github-app-token": "bcd2ba49218906704ab6c1aa796996da409d3eb1", // v3.2.0
}

// RunnerLabels are the only permitted runs-on images (ADR-0058).
var RunnerLabels = map[string]string{
	"ubuntu-24.04": "ubuntu-24.04",
	"windows-2022": "windows-2022",
}

// GcloudPin is the pinned Google Cloud SDK tarball (device-perf only).
var GcloudPin = InstallerPin{
	URL:    "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-586.0.0-linux-x86_64.tar.gz",
	SHA256: "6c774c76793eedd501150b59da653610fbe3eaac169e822965b722de75a2f001",
}
