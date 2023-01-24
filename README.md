# aws-resolver-rules-operator

This operator reconciles `AWSClusters`. It only reconciles `AWSClusters` using the private DNS mode, set by the `aws.giantswarm.io/dns-mode` annotation.
It creates a Resolver Rule for the workload cluster k8s API endpoint and associates it with the DNS Server VPC.
That way, other clients using the DNS Server will be able to resolve the workload cluster k8s API endpoint.
