package registry

import (
	devspacecontext "github.com/loft-sh/devspace/pkg/devspace/context"
	"github.com/loft-sh/devspace/pkg/util/ptr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsapplyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
)

func (r *LocalRegistry) ensureDeployment(ctx devspacecontext.Context) (*appsv1.Deployment, error) {
	// Switching from a persistent registry, delete the statefulset.
	_, err := ctx.KubeClient().KubeClient().AppsV1().StatefulSets(r.options.Namespace).Get(ctx.Context(), r.options.Name, metav1.GetOptions{})
	if err == nil {
		err := ctx.KubeClient().KubeClient().AppsV1().StatefulSets(r.options.Namespace).Delete(ctx.Context(), r.options.Name, metav1.DeleteOptions{})
		if err != nil && kerrors.IsNotFound(err) {
			return nil, err
		}
	}

	// Create if it does not exist
	desired := r.getDeployment()
	existing, err := ctx.KubeClient().KubeClient().AppsV1().Deployments(r.options.Namespace).Get(ctx.Context(), r.options.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return ctx.KubeClient().KubeClient().AppsV1().Deployments(r.options.Namespace).Create(ctx.Context(), desired, metav1.CreateOptions{})
		}

		return nil, err
	}

	// Use server side apply if it does exist
	applyConfiguration, err := appsapplyv1.ExtractDeployment(existing, ApplyFieldManager)
	if err != nil {
		return nil, err
	}
	return ctx.KubeClient().KubeClient().AppsV1().Deployments(r.options.Namespace).Apply(
		ctx.Context(),
		applyConfiguration,
		metav1.ApplyOptions{
			FieldManager: ApplyFieldManager,
			Force:        true,
		},
	)
}

func (r *LocalRegistry) getDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.options.Name,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": r.options.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": r.options.Name,
					},
				},
				Spec: corev1.PodSpec{
					EnableServiceLinks: new(bool),
					Containers: []corev1.Container{
						{
							Name:  "registry",
							Image: r.options.Image,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v2/",
										Port: intstr.IntOrString{
											IntVal: int32(r.options.Port),
										},
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      1,
								PeriodSeconds:       20,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v2/",
										Port: intstr.IntOrString{
											IntVal: int32(r.options.Port),
										},
									},
								},
								InitialDelaySeconds: 2,
								TimeoutSeconds:      1,
								PeriodSeconds:       5,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                ptr.Int64(1000),
								RunAsNonRoot:             ptr.Bool(true),
								ReadOnlyRootFilesystem:   ptr.Bool(true),
								AllowPrivilegeEscalation: new(bool),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "registry",
									MountPath: "/var/lib/registry",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
							Name: "registry",
						},
					},
				},
			},
		},
	}
}