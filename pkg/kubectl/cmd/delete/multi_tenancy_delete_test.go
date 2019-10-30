/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package delete

import (
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
)

func TestDeleteObjectByTupleWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, _, rc := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {

			// replication controller with cascade off
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/redis-master-controller" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil

			// secret with cascade on, but no client-side reaper
			case p == "/tenants/test-te/namespaces/test/secrets/mysecret" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil

			default:
				// Ensures no GET is performed when deleting by name
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()
	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("namespace", "test")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"replicationcontrollers/redis-master-controller"})
	if buf.String() != "replicationcontroller/redis-master-controller\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}

	// Test cascading delete of object without client-side reaper doesn't make GET requests
	streams, _, buf, _ = genericclioptions.NewTestIOStreams()
	cmd = NewCmdDelete(tf, streams)
	cmd.Flags().Set("namespace", "test")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"secrets/mysecret"})
	if buf.String() != "secret/mysecret\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

// Tests that DeleteOptions.OrphanDependents is appropriately set while deleting objects.
func TestOrphanDependentsInDeleteObjectWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, _, rc := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	var policy *metav1.DeletionPropagation
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m, b := req.URL.Path, req.Method, req.Body; {

			case p == "/tenants/test-te/namespaces/test/secrets/mysecret" && m == "DELETE" && hasExpectedPropagationPolicy(b, policy):

				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil
			default:
				return nil, nil
			}
		}),
	}

	// DeleteOptions.PropagationPolicy should be Background, when cascade is true (default).
	backgroundPolicy := metav1.DeletePropagationBackground
	policy = &backgroundPolicy
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()
	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("namespace", "test")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"secrets/mysecret"})
	if buf.String() != "secret/mysecret\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}

	// Test that delete options should be set to orphan when cascade is false.
	orphanPolicy := metav1.DeletePropagationOrphan
	policy = &orphanPolicy
	streams, _, buf, _ = genericclioptions.NewTestIOStreams()
	cmd = NewCmdDelete(tf, streams)
	cmd.Flags().Set("namespace", "test")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"secrets/mysecret"})
	if buf.String() != "secret/mysecret\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteNamedObjectWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	cmdtesting.InitTestErrorHandler(t)
	_, _, rc := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {

			// replication controller with cascade off
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/redis-master-controller" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil

			// secret with cascade on, but no client-side reaper
			case p == "/tenants/test-te/namespaces/test/secrets/mysecret" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil

			default:
				// Ensures no GET is performed when deleting by name
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()
	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("namespace", "test")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"replicationcontrollers", "redis-master-controller"})
	if buf.String() != "replicationcontroller/redis-master-controller\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}

	// Test cascading delete of object without client-side reaper doesn't make GET requests
	streams, _, buf, _ = genericclioptions.NewTestIOStreams()
	cmd = NewCmdDelete(tf, streams)
	cmd.Flags().Set("namespace", "test")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"secrets", "mysecret"})
	if buf.String() != "secret/mysecret\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteObjectWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, _, rc := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/redis-master" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}

	streams, _, buf, _ := genericclioptions.NewTestIOStreams()
	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("filename", "../../../../test/e2e/testing-manifests/guestbook/legacy/redis-master-controller.yaml")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	// uses the name from the file, not the response
	if buf.String() != "replicationcontroller/redis-master\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteObjectGraceZeroWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	pods, _, _ := cmdtesting.TestData()

	count := 0
	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			t.Logf("got request %s %s", req.Method, req.URL.Path)
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/pods/nginx" && m == "GET":
				count++
				switch count {
				case 1, 2, 3:
					return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &pods.Items[0])}, nil
				default:
					return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &metav1.Status{})}, nil
				}
			case p == "/api/v1/namespaces/test" && m == "GET":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &corev1.Namespace{})}, nil
			case p == "/tenants/test-te/namespaces/test/pods/nginx" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &pods.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}

	streams, _, buf, errBuf := genericclioptions.NewTestIOStreams()
	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("output", "name")
	cmd.Flags().Set("grace-period", "0")
	cmd.Run(cmd, []string{"pods/nginx"})

	// uses the name from the file, not the response
	if buf.String() != "pod/nginx\n" {
		t.Errorf("unexpected output: %s\n---\n%s", buf.String(), errBuf.String())
	}
	if count != 0 {
		t.Errorf("unexpected calls to GET: %d", count)
	}
}

func TestDeleteObjectNotFoundWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/redis-master" && m == "DELETE":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.StringBody("")}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}

	options := &DeleteOptions{
		FilenameOptions: resource.FilenameOptions{
			Filenames: []string{"../../../../test/e2e/testing-manifests/guestbook/legacy/redis-master-controller.yaml"},
		},
		GracePeriod: -1,
		Cascade:     false,
		Output:      "name",
		IOStreams:   genericclioptions.NewTestIOStreamsDiscard(),
	}
	err := options.Complete(tf, []string{}, fakecmd())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = options.RunDelete()
	if err == nil || !errors.IsNotFound(err) {
		t.Errorf("unexpected error: expected NotFound, got %v", err)
	}
}

func TestDeleteObjectIgnoreNotFoundWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/redis-master" && m == "DELETE":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.StringBody("")}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("filename", "../../../../test/e2e/testing-manifests/guestbook/legacy/redis-master-controller.yaml")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("ignore-not-found", "true")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	if buf.String() != "" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteAllNotFoundWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, svc, _ := cmdtesting.TestData()
	// Add an item to the list which will result in a 404 on delete
	svc.Items = append(svc.Items, corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo"}})
	notFoundError := &errors.NewNotFound(corev1.Resource("services"), "foo").ErrStatus

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/services" && m == "GET":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, svc)}, nil
			case p == "/tenants/test-te/namespaces/test/services/foo" && m == "DELETE":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, notFoundError)}, nil
			case p == "/tenants/test-te/namespaces/test/services/baz" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &svc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}

	// Make sure we can explicitly choose to fail on NotFound errors, even with --all
	options := &DeleteOptions{
		FilenameOptions: resource.FilenameOptions{},
		GracePeriod:     -1,
		Cascade:         false,
		DeleteAll:       true,
		IgnoreNotFound:  false,
		Output:          "name",
		IOStreams:       genericclioptions.NewTestIOStreamsDiscard(),
	}
	err := options.Complete(tf, []string{"services"}, fakecmd())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = options.RunDelete()
	if err == nil || !errors.IsNotFound(err) {
		t.Errorf("unexpected error: expected NotFound, got %v", err)
	}
}

func TestDeleteAllIgnoreNotFoundWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, svc, _ := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	// Add an item to the list which will result in a 404 on delete
	svc.Items = append(svc.Items, corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo"}})
	notFoundError := &errors.NewNotFound(corev1.Resource("services"), "foo").ErrStatus

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/services" && m == "GET":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, svc)}, nil
			case p == "/tenants/test-te/namespaces/test/services/foo" && m == "DELETE":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, notFoundError)}, nil
			case p == "/tenants/test-te/namespaces/test/services/baz" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &svc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("all", "true")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"services"})

	if buf.String() != "service/baz\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteMultipleObjectWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, svc, rc := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/redis-master" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil
			case p == "/tenants/test-te/namespaces/test/services/frontend" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &svc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("filename", "../../../../test/e2e/testing-manifests/guestbook/legacy/redis-master-controller.yaml")
	cmd.Flags().Set("filename", "../../../../test/e2e/testing-manifests/guestbook/frontend-service.yaml")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	if buf.String() != "replicationcontroller/redis-master\nservice/frontend\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteMultipleObjectContinueOnMissingWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, svc, _ := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/redis-master" && m == "DELETE":
				return &http.Response{StatusCode: 404, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.StringBody("")}, nil
			case p == "/tenants/test-te/namespaces/test/services/frontend" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &svc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	options := &DeleteOptions{
		FilenameOptions: resource.FilenameOptions{
			Filenames: []string{"../../../../test/e2e/testing-manifests/guestbook/legacy/redis-master-controller.yaml", "../../../../test/e2e/testing-manifests/guestbook/frontend-service.yaml"},
		},
		GracePeriod: -1,
		Cascade:     false,
		Output:      "name",
		IOStreams:   streams,
	}
	err := options.Complete(tf, []string{}, fakecmd())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = options.RunDelete()
	if err == nil || !errors.IsNotFound(err) {
		t.Errorf("unexpected error: expected NotFound, got %v", err)
	}

	if buf.String() != "service/frontend\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteMultipleResourcesWithTheSameNameWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, svc, rc := cmdtesting.TestData()
	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/baz" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil
			case p == "/tenants/test-te/namespaces/test/replicationcontrollers/foo" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil
			case p == "/tenants/test-te/namespaces/test/services/baz" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &svc.Items[0])}, nil
			case p == "/tenants/test-te/namespaces/test/services/foo" && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &svc.Items[0])}, nil
			default:
				// Ensures no GET is performed when deleting by name
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("namespace", "test")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"replicationcontrollers,services", "baz", "foo"})
	if buf.String() != "replicationcontroller/baz\nreplicationcontroller/foo\nservice/baz\nservice/foo\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteDirectoryWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	_, _, rc := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case strings.HasPrefix(p, "/tenants/test-te/namespaces/test/replicationcontrollers/") && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &rc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("filename", "../../../../test/e2e/testing-manifests/guestbook/legacy")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{})

	if buf.String() != "replicationcontroller/frontend\nreplicationcontroller/redis-master\nreplicationcontroller/redis-slave\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestDeleteMultipleSelectorWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	pods, svc, _ := cmdtesting.TestData()

	tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
	defer tf.Cleanup()

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == "/tenants/test-te/namespaces/test/pods" && m == "GET":
				if req.URL.Query().Get(metav1.LabelSelectorQueryParam("v1")) != "a=b" {
					t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				}
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, pods)}, nil
			case p == "/tenants/test-te/namespaces/test/services" && m == "GET":
				if req.URL.Query().Get(metav1.LabelSelectorQueryParam("v1")) != "a=b" {
					t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				}
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, svc)}, nil
			case strings.HasPrefix(p, "/tenants/test-te/namespaces/test/pods/") && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &pods.Items[0])}, nil
			case strings.HasPrefix(p, "/tenants/test-te/namespaces/test/services/") && m == "DELETE":
				return &http.Response{StatusCode: 200, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, &svc.Items[0])}, nil
			default:
				t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
				return nil, nil
			}
		}),
	}
	streams, _, buf, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdDelete(tf, streams)
	cmd.Flags().Set("selector", "a=b")
	cmd.Flags().Set("cascade", "false")
	cmd.Flags().Set("output", "name")
	cmd.Run(cmd, []string{"pods,services"})

	if buf.String() != "pod/foo\npod/bar\nservice/baz\n" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestResourceErrorsWithMultiTenancy(t *testing.T) {
	cmdtesting.InitTestErrorHandler(t)
	testCases := map[string]struct {
		args  []string
		errFn func(error) bool
	}{
		"no args": {
			args:  []string{},
			errFn: func(err error) bool { return strings.Contains(err.Error(), "You must provide one or more resources") },
		},
		"resources but no selectors": {
			args: []string{"pods"},
			errFn: func(err error) bool {
				return strings.Contains(err.Error(), "resource(s) were provided, but no name, label selector, or --all flag specified")
			},
		},
		"multiple resources but no selectors": {
			args: []string{"pods,deployments"},
			errFn: func(err error) bool {
				return strings.Contains(err.Error(), "resource(s) were provided, but no name, label selector, or --all flag specified")
			},
		},
	}

	for k, testCase := range testCases {
		t.Run(k, func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespaceWithMultiTenancy("test", "test-te")
			defer tf.Cleanup()

			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()

			streams, _, buf, _ := genericclioptions.NewTestIOStreams()
			options := &DeleteOptions{
				FilenameOptions: resource.FilenameOptions{},
				GracePeriod:     -1,
				Cascade:         false,
				Output:          "name",
				IOStreams:       streams,
			}
			err := options.Complete(tf, testCase.args, fakecmd())
			if !testCase.errFn(err) {
				t.Errorf("%s: unexpected error: %v", k, err)
				return
			}

			if buf.Len() > 0 {
				t.Errorf("buffer should be empty: %s", string(buf.Bytes()))
			}
		})
	}
}
