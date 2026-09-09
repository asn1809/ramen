// SPDX-FileCopyrightText: The RamenDR authors
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ramendr/ramen/internal/controller/util"
)

var _ = Describe("PVCS_Util", func() {
	var (
		testNamespace *corev1.Namespace
		testCtx       context.Context
		cancel        context.CancelFunc
	)

	BeforeEach(func() {
		testCtx, cancel = context.WithCancel(context.TODO())

		// Create namespace for test
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pvc-util-test-ns-",
			},
		}
		Expect(k8sClient.Create(testCtx, testNamespace)).To(Succeed())
		Expect(testNamespace.GetName()).NotTo(BeEmpty())
	})

	AfterEach(func() {
		// All resources are namespaced, so this should clean it all up
		Expect(k8sClient.Delete(testCtx, testNamespace)).To(Succeed())

		cancel()
	})

	Describe("List PVCs by PVCSelector", func() {
		var (
			pvcA *corev1.PersistentVolumeClaim
			pvcB *corev1.PersistentVolumeClaim
			pvcC *corev1.PersistentVolumeClaim
			pvcD *corev1.PersistentVolumeClaim
		)

		pvcCount := 4

		BeforeEach(func() {
			// Create some PVCs
			pvcA = createTestPVC(testCtx, testNamespace.GetName(),
				map[string]string{
					"test-label":           "aaa",
					util.CreatedByLabelKey: util.CreatedByLabelValueVolSync, // Created by volsync
					"mylabel":              "abc",
					"new-label":            "test",
				})

			pvcB = createTestPVC(testCtx, testNamespace.GetName(),
				map[string]string{
					"test-label": "bbb",
					"another":    "somethingelse",
					"new-label":  "test",
				})

			pvcC = createTestPVC(testCtx, testNamespace.GetName(),
				map[string]string{
					"test-label":           "ccc",
					util.CreatedByLabelKey: util.CreatedByLabelValueVolSync, // Created by volsync
					"mylabel":              "abc",
					"another":              "whynot",
					"new-label":            "test",
				})

			pvcD = createTestPVC(testCtx, testNamespace.GetName(),
				map[string]string{
					"test-label": "ddd",
					"mylabel":    "abc",
					"another":    "whynot",
				})
		})

		Context("When labelSelector is empty", func() {
			var pvcSelector metav1.LabelSelector

			It("Should list all PVCs when VolSync is disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					true /* Volsync Disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(pvcCount))
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcA.GetName()),
					HavePVCName(pvcB.GetName()),
					HavePVCName(pvcC.GetName()),
					HavePVCName(pvcD.GetName()),
				))
			})

			It("Should filter out VolSync PVCs when VolSync is not disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					false /* Volsync NOT disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(pvcCount - 2)) // 2 PVCs are VolSync PVCs
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcB.GetName()),
					HavePVCName(pvcD.GetName()),
				))
			})
		})

		Context("With a labelSelector with matchLabels", func() {
			pvcSelector := metav1.LabelSelector{
				MatchLabels: map[string]string{
					"mylabel": "abc", // Matches pvcA, pvcC, pvcD
				},
			}

			It("Should list matching PVCs when VolSync is disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					true /* Volsync Disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(3))
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcA.GetName()),
					HavePVCName(pvcC.GetName()),
					HavePVCName(pvcD.GetName()),
				))
			})

			It("Should list matching PVCs and filter out VolSync PVCs when VolSync is not disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					false /* Volsync NOT Disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(1))
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcD.GetName()),
				))
			})
		})

		Context("With a labelSelector with multiple matchLabels", func() {
			pvcSelector := metav1.LabelSelector{
				MatchLabels: map[string]string{
					"mylabel": "abc",    // Matches pvcA, pvcC, pvcD
					"another": "whynot", // Matches pvcC, pvcD
				},
			}

			It("Should list matching PVCs when VolSync is disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					true /* Volsync Disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(2))
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcC.GetName()),
					HavePVCName(pvcD.GetName()),
				))
			})

			It("Should list matching PVCs and filter out VolSync PVCs when VolSync is not disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					false /* Volsync NOT Disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(1))
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcD.GetName()),
				))
			})
		})

		Context("With a labelSelector with matchLabels and matchExpresssions", func() {
			pvcSelector := metav1.LabelSelector{
				MatchLabels: map[string]string{
					"new-label": "test", // Matches pvcA, pvcB, pvcC
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "test-label",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"bbb", "ddd", "ccc"}, // Should match pvcB, pvcC, pvcD
					},
				},
			}
			// Overall this selector should AND matchLabels and MatchExpresssions, so should match pvcB & pvcC

			It("Should list matching PVCs when VolSync is disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					true /* Volsync Disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(2))
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcB.GetName()),
					HavePVCName(pvcC.GetName()),
				))
			})

			It("Should list matching PVCs and filter out VolSync PVCs when VolSync is not disabled", func() {
				pvcList, err := util.ListPVCsByPVCSelector(testCtx, k8sClient, testLogger, pvcSelector,
					[]string{testNamespace.GetName()},
					false /* Volsync NOT Disabled */)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcList).NotTo(BeNil())
				Expect(len(pvcList.Items)).To(Equal(1))
				Expect(pvcList.Items).Should(ConsistOf(
					HavePVCName(pvcB.GetName()),
				))
			})
		})
	})

	Describe("IsPVCMountedWithSubPath", func() {
		var (
			pvcWithSubPath    *corev1.PersistentVolumeClaim
			pvcWithoutSubPath *corev1.PersistentVolumeClaim
		)

		BeforeEach(func() {
			pvcWithSubPath = createTestPVC(testCtx, testNamespace.GetName(), nil)
			pvcWithoutSubPath = createTestPVC(testCtx, testNamespace.GetName(), nil)
		})

		It("returns true when pod container uses subPath", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-subpath",
					Namespace: testNamespace.GetName(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol-subpath",
									MountPath: "/data",
									SubPath:   "mysubdir",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol-subpath",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcWithSubPath.GetName(),
								},
							},
						},
					},
				},
			}
			cl := fakeclient.NewClientBuilder().
				WithIndex(&corev1.Pod{}, util.PodVolumePVCClaimIndexName, func(obj client.Object) []string {
					pod, ok := obj.(*corev1.Pod)
					if !ok {
						return nil
					}

					var res []string

					for _, vol := range pod.Spec.Volumes {
						if vol.PersistentVolumeClaim != nil {
							res = append(res, vol.PersistentVolumeClaim.ClaimName)
						}
					}

					return res
				}).
				WithObjects(pod).
				Build()

			mountedWithSubPath, err := util.IsPVCMountedWithSubPath(testCtx, cl, testLogger,
				types.NamespacedName{Namespace: pvcWithSubPath.Namespace, Name: pvcWithSubPath.Name})
			Expect(err).NotTo(HaveOccurred())
			Expect(mountedWithSubPath).To(BeTrue())
		})

		It("returns true when pod initContainer uses subPathExpr", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-with-subpathexpr",
					Namespace: testNamespace.GetName(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  "init-app",
							Image: "busybox",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:        "vol-subpathexpr",
									MountPath:   "/data",
									SubPathExpr: "$(POD_NAME)",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol-subpathexpr",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcWithSubPath.GetName(),
								},
							},
						},
					},
				},
			}
			cl := fakeclient.NewClientBuilder().
				WithIndex(&corev1.Pod{}, util.PodVolumePVCClaimIndexName, func(obj client.Object) []string {
					pod, ok := obj.(*corev1.Pod)
					if !ok {
						return nil
					}

					var res []string

					for _, vol := range pod.Spec.Volumes {
						if vol.PersistentVolumeClaim != nil {
							res = append(res, vol.PersistentVolumeClaim.ClaimName)
						}
					}

					return res
				}).
				WithObjects(pod).
				Build()

			mountedWithSubPath, err := util.IsPVCMountedWithSubPath(testCtx, cl, testLogger,
				types.NamespacedName{Namespace: pvcWithSubPath.Namespace, Name: pvcWithSubPath.Name})
			Expect(err).NotTo(HaveOccurred())
			Expect(mountedWithSubPath).To(BeTrue())
		})

		It("returns false when pod mounts pvc directly without subPath", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-without-subpath",
					Namespace: testNamespace.GetName(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vol-direct",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vol-direct",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcWithoutSubPath.GetName(),
								},
							},
						},
					},
				},
			}
			cl := fakeclient.NewClientBuilder().
				WithIndex(&corev1.Pod{}, util.PodVolumePVCClaimIndexName, func(obj client.Object) []string {
					pod, ok := obj.(*corev1.Pod)
					if !ok {
						return nil
					}

					var res []string

					for _, vol := range pod.Spec.Volumes {
						if vol.PersistentVolumeClaim != nil {
							res = append(res, vol.PersistentVolumeClaim.ClaimName)
						}
					}

					return res
				}).
				WithObjects(pod).
				Build()

			mountedWithSubPath, err := util.IsPVCMountedWithSubPath(testCtx, cl, testLogger,
				types.NamespacedName{Namespace: pvcWithoutSubPath.Namespace, Name: pvcWithoutSubPath.Name})
			Expect(err).NotTo(HaveOccurred())
			Expect(mountedWithSubPath).To(BeFalse())
		})
	})
})

func createTestPVC(ctx context.Context, namespace string, labels map[string]string) *corev1.PersistentVolumeClaim {
	pvc := getTestPVC(namespace, labels)
	Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

	return pvc
}

func getTestPVC(namespace string, labels map[string]string) *corev1.PersistentVolumeClaim {
	// Create dummy PVC with the desired labels
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-pvc-",
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
}

// HavePVCName returns a matcher that expects the PVC to have the given name.
func HavePVCName(name string) gomegatypes.GomegaMatcher {
	return WithTransform(func(pvc corev1.PersistentVolumeClaim) string {
		return pvc.GetName()
	}, Equal(name))
}

func TestPVCHashChangeForResize(t *testing.T) {
	labels := map[string]string{
		"test-label": "test",
	}
	pvc := getTestPVC("testns", labels)
	oldHash := util.HashPVC(pvc)
	pvc.Spec.Resources.Requests["capacity"] = resource.MustParse("2Gi")
	newHash := util.HashPVC(pvc)
	assert.NotEqual(t, oldHash, newHash)
}
