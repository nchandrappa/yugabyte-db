/*
 * Copyright (c) YugaByte, Inc.
 */

package kubernetes

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	ybaclient "github.com/yugabyte/platform-go-client"
	"github.com/yugabyte/yugabyte-db/managed/yba-cli/cmd/provider/providerutil"
	"github.com/yugabyte/yugabyte-db/managed/yba-cli/cmd/util"
	ybaAuthClient "github.com/yugabyte/yugabyte-db/managed/yba-cli/internal/client"
	"github.com/yugabyte/yugabyte-db/managed/yba-cli/internal/formatter"
)

// createK8sProviderCmd represents the provider command
var createK8sProviderCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Kubernetes YugabyteDB Anywhere provider",
	Long:  "Create a Kubernetes provider in YugabyteDB Anywhere",
	PreRun: func(cmd *cobra.Command, args []string) {
		providerNameFlag, err := cmd.Flags().GetString("name")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}
		if len(providerNameFlag) == 0 {
			cmd.Help()
			logrus.Fatalln(
				formatter.Colorize("No provider name found to create\n", formatter.RedColor))
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		authAPI, err := ybaAuthClient.NewAuthAPIClient()
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}
		authAPI.GetCustomerUUID()
		providerName, err := cmd.Flags().GetString("name")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}
		providerCode := util.K8sProviderType

		airgapInstall, err := cmd.Flags().GetBool("airgap-install")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		providerType, err := cmd.Flags().GetString("type")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		imageRegistry, err := cmd.Flags().GetString("image-registry")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		pullSecretFilePath, err := cmd.Flags().GetString("pull-secret-file")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		logrus.Debug("Reading Kubernetes Pull Secret")
		pullSecretContent := util.YAMLtoString(pullSecretFilePath)

		configFilePath, err := cmd.Flags().GetString("kubeconfig-file")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		var configContent string
		if len(configFilePath) > 0 {
			logrus.Debug("Reading Kube Config")
			configContent = util.YAMLtoString(configFilePath)
		}

		storageClass, err := cmd.Flags().GetString("storage-class")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		regions, err := cmd.Flags().GetStringArray("region")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		zones, err := cmd.Flags().GetStringArray("zone")
		if err != nil {
			logrus.Fatalf(formatter.Colorize(err.Error()+"\n", formatter.RedColor))
		}

		requestBody := ybaclient.Provider{
			Code:    util.GetStringPointer(providerCode),
			Name:    util.GetStringPointer(providerName),
			Regions: buildK8sRegions(regions, zones),
			Details: &ybaclient.ProviderDetails{
				AirGapInstall: util.GetBoolPointer(airgapInstall),
				CloudInfo: &ybaclient.CloudInfo{
					Kubernetes: &ybaclient.KubernetesInfo{
						KubernetesImageRegistry:     util.GetStringPointer(imageRegistry),
						KubernetesProvider:          util.GetStringPointer(providerType),
						KubernetesPullSecretContent: util.GetStringPointer(pullSecretContent),
						KubernetesStorageClass:      util.GetStringPointer(storageClass),
						KubeConfigContent:           util.GetStringPointer(configContent),
					},
				},
			},
		}

		rCreate, response, err := authAPI.CreateProvider().
			CreateProviderRequest(requestBody).Execute()
		if err != nil {
			errMessage := util.ErrorFromHTTPResponse(
				response,
				err,
				"Provider: Kubernetes",
				"Create")
			logrus.Fatalf(formatter.Colorize(errMessage.Error()+"\n", formatter.RedColor))
		}

		providerUUID := rCreate.GetResourceUUID()
		taskUUID := rCreate.GetTaskUUID()

		providerutil.WaitForCreateProviderTask(authAPI, providerName, providerUUID, taskUUID)
	},
}

func init() {
	createK8sProviderCmd.Flags().SortFlags = false

	createK8sProviderCmd.Flags().String("type", "",
		"[Required] Kubernetes cloud type. Allowed values: aks, eks, gke, custom.")
	createK8sProviderCmd.MarkFlagRequired(util.K8sProviderType)
	createK8sProviderCmd.Flags().String("image-registry", "quay.io/yugabyte/yugabyte",
		"[Optional] Kubernetes Image Registry.")

	createK8sProviderCmd.Flags().String("pull-secret-file", "",
		"[Required] Kuberenetes Pull Secret File Path.")
	createK8sProviderCmd.MarkFlagRequired("pull-secret-file")

	createK8sProviderCmd.Flags().String("kubeconfig-file", "",
		"[Optional] Kuberenetes Config File Path.")

	createK8sProviderCmd.Flags().String("storage-class", "",
		"[Optional] Kubernetes Storage Class.")

	createK8sProviderCmd.Flags().StringArray("region", []string{},
		"[Required] Region associated with the Kubernetes provider. Minimum number of required "+
			"regions = 1. Provide the following comma separated fields as key-value pairs:"+
			"\"region-name=<region-name>,config-name=<config-name>,"+
			"config-file-path=<path-for-the-kubernetes-region-config-file>,"+
			"storage-class=<storage-class>,"+
			"cert-manager-cluster-issuer=<cert-manager-cluster-issuer>,"+
			"cert-manager-issuer=<cert-manager-issuer>,domain=<domain>,namespace=<namespace>,"+
			"pod-address-template=<pod-address-template>,"+
			"overrirdes-file-path=<path-for-file-contanining-overries>\". "+
			formatter.Colorize("Region name is a required key-value.",
				formatter.GreenColor)+
			" Config name (and Config File Path provided together), Storage Class, Cert Manager"+
			" Cluster Issuer, Cert Manager Issuer, Domain, Namespace, Pod Address Template and"+
			" Overrides File Path are optional. "+
			"Each region needs to be added using a separate --region flag.")

	createK8sProviderCmd.Flags().StringArray("zone", []string{},
		"[Required] Zone associated to the Kubernetes Region defined. "+
			"Provide the following comma separated fields as key-value pairs:"+
			"\"zone-name=<zone-name>,region-name=<region-name>,config-name=<config-name>,"+
			"config-file-path=<path-for-the-kubernetes-region-config-file>,"+
			"storage-class=<storage-class>,"+
			"cert-manager-cluster-issuer=<cert-manager-cluster-issuer>,"+
			"cert-manager-issuer=<cert-manager-issuer>,domain=<domain>,namespace=<namespace>,"+
			"pod-address-template=<pod-address-template>,"+
			"overrirdes-file-path=<path-for-file-contanining-overries>\". "+
			formatter.Colorize("Zone name and Region name are required values. ",
				formatter.GreenColor)+
			" Config name (and Config File Path provided together), Storage Class, Cert Manager"+
			" Cluster Issuer, Cert Manager Issuer, Domain, Namespace, Pod Address Template and"+
			" Overrides File Path are optional. "+
			"Each --region definition "+
			"must have atleast one corresponding --zone definition. Multiple --zone definitions "+
			"can be provided per region."+
			"Each zone needs to be added using a separate --zone flag.")
	createK8sProviderCmd.Flags().Bool("airgap-install", false,
		"[Optional] Do YugabyteDB nodes have access to public internet to download packages.")
}

func buildK8sRegions(
	regionStrings,
	zoneStrings []string) (res []ybaclient.Region) {
	if len(regionStrings) == 0 {
		logrus.Fatalln(
			formatter.Colorize("Atleast one region is required per provider.",
				formatter.RedColor))
	}
	for _, regionString := range regionStrings {
		region := map[string]string{}
		for _, regionInfo := range strings.Split(regionString, ",") {
			kvp := strings.Split(regionInfo, "=")
			if len(kvp) != 2 {
				logrus.Fatalln(
					formatter.Colorize("Incorrect format in region description.",
						formatter.RedColor))
			}
			key := kvp[0]
			val := kvp[1]
			switch key {
			case "region-name":
				if len(strings.TrimSpace(val)) != 0 {
					region["name"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "config-name":
				if len(strings.TrimSpace(val)) != 0 {
					region["config-name"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "config-file-path":
				if len(strings.TrimSpace(val)) != 0 {
					region["config-file-path"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "storage-class":
				if len(strings.TrimSpace(val)) != 0 {
					region["storage-class"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "cert-manager-cluster-issuer":
				if len(strings.TrimSpace(val)) != 0 {
					region["cert-manager-cluster-issuer"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "cert-manager-issuer":
				if len(strings.TrimSpace(val)) != 0 {
					region["cert-manager-issuer"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "domain":
				if len(strings.TrimSpace(val)) != 0 {
					region["domain"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "namespace":
				if len(strings.TrimSpace(val)) != 0 {
					region["namespace"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "overrirdes-file-path":
				if len(strings.TrimSpace(val)) != 0 {
					region["overrirdes-file-path"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "pod-address-template":
				if len(strings.TrimSpace(val)) != 0 {
					region["pod-address-template"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			}
		}
		if _, ok := region["name"]; !ok {
			logrus.Fatalln(
				formatter.Colorize("Name not specified in region.",
					formatter.RedColor))
		}
		var configContent string
		if _, ok := region["config-name"]; ok {
			if _, ok := region["config-file-path"]; !ok {
				logrus.Fatalln(
					formatter.Colorize("Config file path not specified in region.",
						formatter.RedColor))
			} else {
				logrus.Debug("Reading Region Kube Config")
				configContent = util.YAMLtoString(region["config-file-path"])
			}
		}

		var overrides string
		if _, ok := region["overrirdes-file-path"]; ok {
			logrus.Debug("Reading Region Kubernetes Overrides")
			overrides = util.YAMLtoString(region["overrirdes-file-path"])
		}

		zones := buildK8sZones(zoneStrings, region["name"])
		r := ybaclient.Region{
			Code: util.GetStringPointer(region["name"]),
			Name: util.GetStringPointer(region["name"]),
			Details: &ybaclient.RegionDetails{
				CloudInfo: &ybaclient.RegionCloudInfo{
					Kubernetes: &ybaclient.KubernetesRegionInfo{
						CertManagerClusterIssuer: util.GetStringPointer(region["cert-manager-cluster-issuer"]),
						CertManagerIssuer:        util.GetStringPointer(region["cert-manager-issuer"]),
						KubeConfigContent:        util.GetStringPointer(configContent),
						KubeDomain:               util.GetStringPointer(region["domain"]),
						KubeNamespace:            util.GetStringPointer(region["namespace"]),
						KubePodAddressTemplate:   util.GetStringPointer(region["pod-address-template"]),
						KubernetesStorageClass:   util.GetStringPointer(region["storage-class"]),
						Overrides:                util.GetStringPointer(overrides),
					},
				},
			},
			Zones: zones,
		}
		res = append(res, r)
	}
	return res
}

func buildK8sZones(
	zoneStrings []string,
	regionName string) (res []ybaclient.AvailabilityZone) {
	for _, zoneString := range zoneStrings {
		zone := map[string]string{}
		for _, zoneInfo := range strings.Split(zoneString, ",") {
			kvp := strings.Split(zoneInfo, "=")
			if len(kvp) != 2 {
				logrus.Fatalln(
					formatter.Colorize("Incorrect format in zone description",
						formatter.RedColor))
			}
			key := kvp[0]
			val := kvp[1]
			switch key {
			case "zone-name":
				if len(strings.TrimSpace(val)) != 0 {
					zone["name"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "region-name":
				if len(strings.TrimSpace(val)) != 0 {
					zone["region-name"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "config-name":
				if len(strings.TrimSpace(val)) != 0 {
					zone["config-name"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "config-file-path":
				if len(strings.TrimSpace(val)) != 0 {
					zone["config-file-path"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "storage-class":
				if len(strings.TrimSpace(val)) != 0 {
					zone["storage-class"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "cert-manager-cluster-issuer":
				if len(strings.TrimSpace(val)) != 0 {
					zone["cert-manager-cluster-issuer"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "cert-manager-issuer":
				if len(strings.TrimSpace(val)) != 0 {
					zone["cert-manager-issuer"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "domain":
				if len(strings.TrimSpace(val)) != 0 {
					zone["domain"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "namespace":
				if len(strings.TrimSpace(val)) != 0 {
					zone["namespace"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "overrirdes-file-path":
				if len(strings.TrimSpace(val)) != 0 {
					zone["overrirdes-file-path"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			case "pod-address-template":
				if len(strings.TrimSpace(val)) != 0 {
					zone["pod-address-template"] = val
				} else {
					providerutil.ValueNotFoundForKeyError(key)
				}
			}
		}
		if _, ok := zone["name"]; !ok {
			logrus.Fatalln(
				formatter.Colorize("Name not specified in zone.",
					formatter.RedColor))
		}
		if _, ok := zone["region-name"]; !ok {
			logrus.Fatalln(
				formatter.Colorize("Region name not specified in zone.",
					formatter.RedColor))
		}

		var configContent string
		if _, ok := zone["config-name"]; ok {
			if _, ok := zone["config-file-path"]; !ok {
				logrus.Fatalln(
					formatter.Colorize("Config file path not specified in zone.",
						formatter.RedColor))
			} else {
				logrus.Debug("Reading Zone Kube Config")
				configContent = util.YAMLtoString(zone["config-file-path"])
			}
		}

		var overrides string
		if _, ok := zone["overrirdes-file-path"]; ok {
			logrus.Debug("Reading Zone Kubernetes Overrides")
			overrides = util.YAMLtoString(zone["overrirdes-file-path"])
		}

		if strings.Compare(zone["region-name"], regionName) == 0 {
			z := ybaclient.AvailabilityZone{
				Code: util.GetStringPointer(zone["name"]),
				Name: zone["name"],
				Details: &ybaclient.AvailabilityZoneDetails{
					CloudInfo: &ybaclient.AZCloudInfo{
						Kubernetes: &ybaclient.KubernetesRegionInfo{
							CertManagerClusterIssuer: util.GetStringPointer(zone["cert-manager-cluster-issuer"]),
							CertManagerIssuer:        util.GetStringPointer(zone["cert-manager-issuer"]),
							KubeConfigContent:        util.GetStringPointer(configContent),
							KubeDomain:               util.GetStringPointer(zone["domain"]),
							KubeNamespace:            util.GetStringPointer(zone["namespace"]),
							KubePodAddressTemplate:   util.GetStringPointer(zone["pod-address-template"]),
							KubernetesStorageClass:   util.GetStringPointer(zone["storage-class"]),
							Overrides:                util.GetStringPointer(overrides),
						},
					},
				},
			}
			res = append(res, z)
		}
	}
	if len(res) == 0 {
		logrus.Fatalln(
			formatter.Colorize("Atleast one zone is required per region.",
				formatter.RedColor))
	}
	return res
}