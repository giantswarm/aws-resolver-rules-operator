package acceptance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws-resolver-rules-operator/tests"
	"github.com/aws-resolver-rules-operator/tests/acceptance/fixture"
)

const (
	DefaultMaxLogLines     = 300
	DefaultPollingInterval = 5 * time.Second
	DefaultTimeout         = 3 * time.Minute
)

var (
	k8sClient client.Client

	namespace    string
	namespaceObj *corev1.Namespace

	testFixture *fixture.Fixture
)

func LogCollectorFailHandler(message string, callerSkip ...int) {
	getPodLogs()
	Fail(message, callerSkip...)
}

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(LogCollectorFailHandler)
	RunSpecs(t, "Acceptance Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	setupEventuallyParameters()
	k8sClient = setupKubeClient()

	fixtureConfig := getFixtureConfig()
	testFixture = fixture.NewFixture(k8sClient, fixtureConfig)

	DeferCleanup(func() {
		Expect(testFixture.Teardown()).To(Succeed())
	})
	err := testFixture.Setup()
	Expect(err).NotTo(HaveOccurred())

	data := fixture.Data{
		Config:  fixtureConfig,
		Network: testFixture.Network,
	}
	encoded, err := json.Marshal(data)
	Expect(err).NotTo(HaveOccurred())

	return encoded
}, func(jsonData []byte) {
	data := fixture.Data{}
	err := json.Unmarshal(jsonData, &data)
	Expect(err).NotTo(HaveOccurred())

	setupEventuallyParameters()
	k8sClient = setupKubeClient()

	testFixture = fixture.LoadFixture(k8sClient, data)
})

var _ = BeforeEach(func() {
	namespace = uuid.New().String()
	namespaceObj = &corev1.Namespace{}
	namespaceObj.Name = namespace
	Expect(k8sClient.Create(context.Background(), namespaceObj)).To(Succeed())
})

var _ = AfterEach(func() {
	Expect(k8sClient.Delete(context.Background(), namespaceObj)).To(Succeed())
})

func getDurationFromEnvVar(env string, defaultDuration time.Duration) time.Duration {
	stringDuration := os.Getenv(env)
	if stringDuration == "" {
		return defaultDuration
	}

	duration, err := time.ParseDuration(stringDuration)
	Expect(err).NotTo(HaveOccurred())

	return duration
}

func getMaxLogLines() int64 {
	maxLogLines := os.Getenv("MAX_LOG_LINES")
	if maxLogLines == "" {
		return DefaultMaxLogLines
	}

	maxLogLinesInt, err := strconv.Atoi(maxLogLines)
	Expect(err).NotTo(HaveOccurred())

	return int64(maxLogLinesInt)
}

func getPodLogs() {
	kubeConfigPath := tests.GetEnvOrSkip("KUBECONFIG")
	maxLogLines := getMaxLogLines()
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		GinkgoWriter.Printf("Failed to build client config: %v", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		GinkgoWriter.Printf("Failed to build client: %v", err)
		return
	}

	ctx := context.Background()
	podsClient := clientset.CoreV1().Pods("giantswarm")
	pods, err := podsClient.List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=aws-resolver-rules-operator",
	})
	if err != nil {
		GinkgoWriter.Printf("Failed to list pods: %v", err)
		return
	}

	if len(pods.Items) == 0 {
		GinkgoWriter.Printf("No pods found: %v", err)
		return
	}
	pod := pods.Items[0]

	logOptions := corev1.PodLogOptions{TailLines: awssdk.Int64(maxLogLines)}
	req := podsClient.GetLogs(pod.Name, &logOptions)
	logStream, err := req.Stream(context.Background())
	if err != nil {
		GinkgoWriter.Printf("Failed to get log stream: %v", err)
		return
	}
	defer func() {
		if err := logStream.Close(); err != nil {
			GinkgoWriter.Printf("failed to close log stream: %v", err)
		}
	}()

	logBytes, err := io.ReadAll(logStream)
	if err != nil {
		GinkgoWriter.Printf("Failed to read logs: %v", err)
		return
	}

	GinkgoWriter.Printf(
		"\n\n---------------- Last %d log lines for %q pod ----------------\n%s\n--------------------------------\n\n",
		maxLogLines,
		pod.Name,
		string(logBytes))
}

func getFixtureConfig() fixture.Config {
	awsAccount := tests.GetEnvOrSkip("MC_AWS_ACCOUNT")
	iamRoleID := tests.GetEnvOrSkip("AWS_IAM_ROLE_ID")
	awsRegion := tests.GetEnvOrSkip("AWS_REGION")
	managementClusterName := tests.GetEnvOrSkip("MANAGEMENT_CLUSTER_NAME")
	managementClusterNamespace := tests.GetEnvOrSkip("MANAGEMENT_CLUSTER_NAMESPACE")
	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", awsAccount, iamRoleID)

	return fixture.Config{
		AWSAccount:                 awsAccount,
		AWSIAMRoleARN:              roleARN,
		AWSRegion:                  awsRegion,
		ManagementClusterName:      managementClusterName,
		ManagementClusterNamespace: managementClusterNamespace,
	}
}

func setupEventuallyParameters() {
	pollingInterval := getDurationFromEnvVar("EVENTUALLY_POLLING_INTERVAL", DefaultPollingInterval)
	timeout := getDurationFromEnvVar("EVENTUALLY_TIMEOUT", DefaultTimeout)

	SetDefaultEventuallyPollingInterval(pollingInterval)
	SetDefaultEventuallyTimeout(timeout)
}

func setupKubeClient() client.Client {
	tests.GetEnvOrSkip("KUBECONFIG")

	config, err := controllerruntime.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	err = capa.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = capi.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	return k8sClient
}
