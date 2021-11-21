package main

import (
	"net"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/client-go/tools/clusterid"
	auditorapi "kmodules.xyz/custom-resources/apis/auditor/v1alpha1"
)

func GetSiteInfo(cfg *rest.Config, kc kubernetes.Interface) (*auditorapi.SiteInfo, error) {
	si := auditorapi.SiteInfo{
		TypeMeta: metav1.TypeMeta{
			APIVersion: auditorapi.SchemeGroupVersion.String(),
			Kind:       "SiteInfo",
		},
	}

	var err error
	si.Kubernetes.ClusterName = clusterid.ClusterName()
	si.Kubernetes.ClusterUID, err = clusterid.ClusterUID(kc.CoreV1().Namespaces())
	if err != nil {
		return nil, err
	}
	si.Kubernetes.Version, err = kc.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	cert, err := meta_util.APIServerCertificate(cfg)
	if err != nil {
		return nil, err
	} else {
		si.Kubernetes.ControlPlane = &auditorapi.ControlPlaneInfo{
			NotBefore: metav1.NewTime(cert.NotBefore),
			NotAfter:  metav1.NewTime(cert.NotAfter),
			// DNSNames:       cert.DNSNames,
			EmailAddresses: cert.EmailAddresses,
			// IPAddresses:    cert.IPAddresses,
			// URIs:           cert.URIs,
		}

		dnsNames := sets.NewString(cert.DNSNames...)
		ips := sets.NewString()
		if len(cert.Subject.CommonName) > 0 {
			if ip := net.ParseIP(cert.Subject.CommonName); ip != nil {
				if !skipIP(ip) {
					ips.Insert(ip.String())
				}
			} else {
				dnsNames.Insert(cert.Subject.CommonName)
			}
		}

		for _, host := range dnsNames.UnsortedList() {
			if host == "kubernetes" ||
				host == "kubernetes.default" ||
				host == "kubernetes.default.svc" ||
				strings.HasSuffix(host, ".svc.cluster.local") ||
				host == "localhost" ||
				!strings.ContainsRune(host, '.') {
				dnsNames.Delete(host)
			}
		}
		si.Kubernetes.ControlPlane.DNSNames = dnsNames.List()

		for _, ip := range cert.IPAddresses {
			if !skipIP(ip) {
				ips.Insert(ip.String())
			}
		}
		si.Kubernetes.ControlPlane.IPAddresses = ips.List()

		uris := make([]string, 0, len(cert.URIs))
		for _, u := range cert.URIs {
			uris = append(uris, u.String())
		}
		si.Kubernetes.ControlPlane.URIs = uris
	}
	return &si, nil
}

func skipIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsMulticast() ||
		ip.IsGlobalUnicast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsLinkLocalUnicast()
}
