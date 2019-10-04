package droptailer

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	podname   = "droptailer"
	image     = "metalpod/droptailer:latest"
	port      = 50051
	namespace = "firewall"
)

// DropTailer is responsible to deploy and watch the droptailer service
type DropTailer struct {
	client    k8s.Interface
	logger    *zap.SugaredLogger
	namespace string
}

// NewDropTailer creates a new DropTailer
func NewDropTailer(logger *zap.SugaredLogger, client k8s.Interface) *DropTailer {
	return &DropTailer{
		client:    client,
		logger:    logger,
		namespace: namespace,
	}
}

// Deploy the DropTailer
func (d *DropTailer) Deploy() error {

	nsSpec := &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: d.namespace}}

	ns, err := d.client.CoreV1().Namespaces().Create(nsSpec)
	if err != nil {
		return fmt.Errorf("unable to create firewall namespace:%w", err)
	}

	deploymentsClient := d.client.AppsV1().Deployments(ns.Name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: podname + "-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": podname,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": podname,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  podname,
							Image: image,
							Ports: []apiv1.ContainerPort{
								{
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: port,
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
		return fmt.Errorf("unable to deploy droptailer:%w", err)
	}
	d.logger.Infow("created deployment", "name", result.GetObjectMeta().GetName())
	return nil
}

// Watch the droptailer, gather pod ip and update /etc/hosts
func (d *DropTailer) Watch() error {
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"app": podname},
	}
	labelMap, err := metav1.LabelSelectorAsMap(labelSelector)
	if err != nil {
		return fmt.Errorf("unable to create labelselector:%w", err)
	}
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

			d.logger.Infof("status:%s", p.Status.ContainerStatuses)
			d.logger.Infof("phase:%s", p.Status.Phase)
			d.logger.Infof("podIP:%s", p.Status.PodIP)
			// TODO update /etc/hosts entry with this podIP
		}
	}
}

func int32Ptr(i int32) *int32 { return &i }
