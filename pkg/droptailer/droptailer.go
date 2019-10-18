package droptailer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/txn2/txeh"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	secretNameDroptailer        = "droptailer-secrets"
	secretNameServerCertificate = "SERVER_CERTIFICATE"
	secretNameServerKey         = "SERVER_KEY"
	defaultCertificateBase      = "/etc/droptailer"
)

// DropTailer is responsible to deploy and watch the droptailer service
type DropTailer struct {
	client          k8s.Interface
	logger          *zap.SugaredLogger
	podname         string
	image           string
	namespace       string
	port            int32
	replicas        int32
	hosts           *txeh.Hosts
	oldPodIP        string
	certificateBase string
}

// NewDropTailer creates a new DropTailer
func NewDropTailer(logger *zap.SugaredLogger, client k8s.Interface) (*DropTailer, error) {
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		return nil, fmt.Errorf("unable to create hosts editor:%w", err)
	}
	certificateBase := os.Getenv("CERTIFICATE_BASE")
	if certificateBase == "" {
		certificateBase = defaultCertificateBase
	}
	return &DropTailer{
		client:          client,
		logger:          logger,
		podname:         "droptailer",
		namespace:       "firewall",
		image:           "metalpod/droptailer:latest",
		port:            50051,
		replicas:        1,
		hosts:           hosts,
		certificateBase: certificateBase,
	}, nil
}

// Deploy the DropTailer
func (d *DropTailer) Deploy() error {
	ns, err := d.client.CoreV1().Namespaces().Get(d.namespace, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("unable to get firewall namespace:%w", err)
	}
	if errors.IsNotFound(err) {
		nsSpec := &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: d.namespace}}
		ns, err = d.client.CoreV1().Namespaces().Create(nsSpec)
		if err != nil {
			return fmt.Errorf("unable to create firewall namespace, err: %w", err)
		}
	}

	// Secret will not be updated
	_, err = d.client.CoreV1().Secrets(d.namespace).Get(secretNameDroptailer, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("unable to get droptailer secret, err: %w", err)
	}
	if errors.IsNotFound(err) {
		serverCert, err := ioutil.ReadFile(path.Join(d.certificateBase, "droptailer-server.pem"))
		if err != nil {
			return fmt.Errorf("could not read server certificate, err: %w", err)
		}
		serverKey, err := ioutil.ReadFile(path.Join(d.certificateBase, "droptailer-server-key.pem"))
		if err != nil {
			return fmt.Errorf("could not read server key, err: %w", err)
		}
		secretsClient := d.client.CoreV1().Secrets(ns.Name)
		secret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretNameDroptailer,
			},
			Data: map[string][]byte{
				secretNameServerCertificate: serverCert,
				secretNameServerKey:         serverKey,
			},
		}
		_, err = secretsClient.Create(secret)
		if err != nil {
			return fmt.Errorf("could not deploy droptailer secrets, err: %v", err)
		}
	}

	deploymentsClient := d.client.AppsV1().Deployments(ns.Name)
	deploymentName := d.podname
	_, err = deploymentsClient.Get(deploymentName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("unable to get droptailer-server deployment, err: %w", err)
	}
	if errors.IsNotFound(err) {
		userid := int64(1000)
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: deploymentName,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &d.replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": d.podname,
					},
				},
				Template: apiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": d.podname,
						},
					},
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:  d.podname,
								Image: d.image,
								Ports: []apiv1.ContainerPort{
									{
										Protocol:      apiv1.ProtocolTCP,
										ContainerPort: d.port,
									},
								},
								Resources: apiv1.ResourceRequirements{
									Limits: apiv1.ResourceList{
										"cpu":    resource.MustParse("200m"),
										"memory": resource.MustParse("128Mi"),
									},
								},
								SecurityContext: &apiv1.SecurityContext{
									RunAsUser: &userid,
								},
								Env: []apiv1.EnvVar{
									{
										Name:  "SERVER_CERTIFICATE",
										Value: "/certificates/server.pem",
									},
									{
										Name:  "SERVER_KEY",
										Value: "/certificates/server-key.pem",
									},
								},
								VolumeMounts: []apiv1.VolumeMount{
									{
										Name:      secretNameDroptailer,
										MountPath: "/certificates/",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []apiv1.Volume{
							{
								Name: secretNameDroptailer,
								VolumeSource: apiv1.VolumeSource{
									Secret: &apiv1.SecretVolumeSource{
										SecretName: secretNameDroptailer,
										Items: []apiv1.KeyToPath{
											{
												Key:  secretNameServerCertificate,
												Path: "server.pem",
											},
											{
												Key:  secretNameServerKey,
												Path: "server-key.pem",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		result, err := deploymentsClient.Create(deployment)
		if err != nil {
			return fmt.Errorf("unable to deploy droptailer-server, err: %w", err)
		}
		d.logger.Infow("created deployment", "name", result.GetObjectMeta().GetName())
	}
	return nil
}

// Watch the droptailer, gather pod ip and update /etc/hosts
func (d *DropTailer) Watch() {
	labelMap := map[string]string{"app": d.podname}
	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap).String(),
	}
	for {
		watcher, err := d.client.CoreV1().Pods(d.namespace).Watch(opts)
		if err != nil {
			d.logger.Errorw("could not watch for services", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}
		for event := range watcher.ResultChan() {
			p, ok := event.Object.(*apiv1.Pod)
			if !ok {
				d.logger.Error("unexpected type")
			}
			podIP := p.Status.PodIP
			if podIP != "" && d.oldPodIP != podIP {
				d.logger.Infow("podIP changed, update /etc/hosts", "old", d.oldPodIP, "new", podIP)
				d.hosts.RemoveHost("droptailer")
				d.hosts.AddHost(p.Status.PodIP, "droptailer")
				d.oldPodIP = podIP
			}
		}
	}
}
