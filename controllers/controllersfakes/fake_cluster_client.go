// Code generated by counterfeiter. DO NOT EDIT.
package controllersfakes

import (
	"context"
	"sync"

	"github.com/aws-resolver-rules-operator/controllers"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	v1beta1a "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1beta1"
	v1beta1b "sigs.k8s.io/cluster-api/api/v1beta1"
)

type FakeClusterClient struct {
	AddAWSClusterFinalizerStub        func(context.Context, *v1beta1.AWSCluster, string) error
	addAWSClusterFinalizerMutex       sync.RWMutex
	addAWSClusterFinalizerArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1.AWSCluster
		arg3 string
	}
	addAWSClusterFinalizerReturns struct {
		result1 error
	}
	addAWSClusterFinalizerReturnsOnCall map[int]struct {
		result1 error
	}
	AddAWSManagedControlPlaneFinalizerStub        func(context.Context, *v1beta1a.AWSManagedControlPlane, string) error
	addAWSManagedControlPlaneFinalizerMutex       sync.RWMutex
	addAWSManagedControlPlaneFinalizerArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1a.AWSManagedControlPlane
		arg3 string
	}
	addAWSManagedControlPlaneFinalizerReturns struct {
		result1 error
	}
	addAWSManagedControlPlaneFinalizerReturnsOnCall map[int]struct {
		result1 error
	}
	AddClusterFinalizerStub        func(context.Context, *v1beta1b.Cluster, string) error
	addClusterFinalizerMutex       sync.RWMutex
	addClusterFinalizerArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1b.Cluster
		arg3 string
	}
	addClusterFinalizerReturns struct {
		result1 error
	}
	addClusterFinalizerReturnsOnCall map[int]struct {
		result1 error
	}
	GetAWSClusterStub        func(context.Context, types.NamespacedName) (*v1beta1.AWSCluster, error)
	getAWSClusterMutex       sync.RWMutex
	getAWSClusterArgsForCall []struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}
	getAWSClusterReturns struct {
		result1 *v1beta1.AWSCluster
		result2 error
	}
	getAWSClusterReturnsOnCall map[int]struct {
		result1 *v1beta1.AWSCluster
		result2 error
	}
	GetAWSManagedControlPlaneStub        func(context.Context, types.NamespacedName) (*v1beta1a.AWSManagedControlPlane, error)
	getAWSManagedControlPlaneMutex       sync.RWMutex
	getAWSManagedControlPlaneArgsForCall []struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}
	getAWSManagedControlPlaneReturns struct {
		result1 *v1beta1a.AWSManagedControlPlane
		result2 error
	}
	getAWSManagedControlPlaneReturnsOnCall map[int]struct {
		result1 *v1beta1a.AWSManagedControlPlane
		result2 error
	}
	GetBastionMachineStub        func(context.Context, string) (*v1beta1b.Machine, error)
	getBastionMachineMutex       sync.RWMutex
	getBastionMachineArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	getBastionMachineReturns struct {
		result1 *v1beta1b.Machine
		result2 error
	}
	getBastionMachineReturnsOnCall map[int]struct {
		result1 *v1beta1b.Machine
		result2 error
	}
	GetClusterStub        func(context.Context, types.NamespacedName) (*v1beta1b.Cluster, error)
	getClusterMutex       sync.RWMutex
	getClusterArgsForCall []struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}
	getClusterReturns struct {
		result1 *v1beta1b.Cluster
		result2 error
	}
	getClusterReturnsOnCall map[int]struct {
		result1 *v1beta1b.Cluster
		result2 error
	}
	GetIdentityStub        func(context.Context, *v1beta1.AWSIdentityReference) (*v1beta1.AWSClusterRoleIdentity, error)
	getIdentityMutex       sync.RWMutex
	getIdentityArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1.AWSIdentityReference
	}
	getIdentityReturns struct {
		result1 *v1beta1.AWSClusterRoleIdentity
		result2 error
	}
	getIdentityReturnsOnCall map[int]struct {
		result1 *v1beta1.AWSClusterRoleIdentity
		result2 error
	}
	MarkConditionTrueStub        func(context.Context, *v1beta1b.Cluster, v1beta1b.ConditionType) error
	markConditionTrueMutex       sync.RWMutex
	markConditionTrueArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1b.Cluster
		arg3 v1beta1b.ConditionType
	}
	markConditionTrueReturns struct {
		result1 error
	}
	markConditionTrueReturnsOnCall map[int]struct {
		result1 error
	}
	RemoveAWSClusterFinalizerStub        func(context.Context, *v1beta1.AWSCluster, string) error
	removeAWSClusterFinalizerMutex       sync.RWMutex
	removeAWSClusterFinalizerArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1.AWSCluster
		arg3 string
	}
	removeAWSClusterFinalizerReturns struct {
		result1 error
	}
	removeAWSClusterFinalizerReturnsOnCall map[int]struct {
		result1 error
	}
	RemoveAWSManagedControlPlaneFinalizerStub        func(context.Context, *v1beta1a.AWSManagedControlPlane, string) error
	removeAWSManagedControlPlaneFinalizerMutex       sync.RWMutex
	removeAWSManagedControlPlaneFinalizerArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1a.AWSManagedControlPlane
		arg3 string
	}
	removeAWSManagedControlPlaneFinalizerReturns struct {
		result1 error
	}
	removeAWSManagedControlPlaneFinalizerReturnsOnCall map[int]struct {
		result1 error
	}
	RemoveClusterFinalizerStub        func(context.Context, *v1beta1b.Cluster, string) error
	removeClusterFinalizerMutex       sync.RWMutex
	removeClusterFinalizerArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1b.Cluster
		arg3 string
	}
	removeClusterFinalizerReturns struct {
		result1 error
	}
	removeClusterFinalizerReturnsOnCall map[int]struct {
		result1 error
	}
	UnpauseStub        func(context.Context, *v1beta1.AWSCluster, *v1beta1b.Cluster) error
	unpauseMutex       sync.RWMutex
	unpauseArgsForCall []struct {
		arg1 context.Context
		arg2 *v1beta1.AWSCluster
		arg3 *v1beta1b.Cluster
	}
	unpauseReturns struct {
		result1 error
	}
	unpauseReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeClusterClient) AddAWSClusterFinalizer(arg1 context.Context, arg2 *v1beta1.AWSCluster, arg3 string) error {
	fake.addAWSClusterFinalizerMutex.Lock()
	ret, specificReturn := fake.addAWSClusterFinalizerReturnsOnCall[len(fake.addAWSClusterFinalizerArgsForCall)]
	fake.addAWSClusterFinalizerArgsForCall = append(fake.addAWSClusterFinalizerArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1.AWSCluster
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.AddAWSClusterFinalizerStub
	fakeReturns := fake.addAWSClusterFinalizerReturns
	fake.recordInvocation("AddAWSClusterFinalizer", []interface{}{arg1, arg2, arg3})
	fake.addAWSClusterFinalizerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) AddAWSClusterFinalizerCallCount() int {
	fake.addAWSClusterFinalizerMutex.RLock()
	defer fake.addAWSClusterFinalizerMutex.RUnlock()
	return len(fake.addAWSClusterFinalizerArgsForCall)
}

func (fake *FakeClusterClient) AddAWSClusterFinalizerCalls(stub func(context.Context, *v1beta1.AWSCluster, string) error) {
	fake.addAWSClusterFinalizerMutex.Lock()
	defer fake.addAWSClusterFinalizerMutex.Unlock()
	fake.AddAWSClusterFinalizerStub = stub
}

func (fake *FakeClusterClient) AddAWSClusterFinalizerArgsForCall(i int) (context.Context, *v1beta1.AWSCluster, string) {
	fake.addAWSClusterFinalizerMutex.RLock()
	defer fake.addAWSClusterFinalizerMutex.RUnlock()
	argsForCall := fake.addAWSClusterFinalizerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) AddAWSClusterFinalizerReturns(result1 error) {
	fake.addAWSClusterFinalizerMutex.Lock()
	defer fake.addAWSClusterFinalizerMutex.Unlock()
	fake.AddAWSClusterFinalizerStub = nil
	fake.addAWSClusterFinalizerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) AddAWSClusterFinalizerReturnsOnCall(i int, result1 error) {
	fake.addAWSClusterFinalizerMutex.Lock()
	defer fake.addAWSClusterFinalizerMutex.Unlock()
	fake.AddAWSClusterFinalizerStub = nil
	if fake.addAWSClusterFinalizerReturnsOnCall == nil {
		fake.addAWSClusterFinalizerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.addAWSClusterFinalizerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) AddAWSManagedControlPlaneFinalizer(arg1 context.Context, arg2 *v1beta1a.AWSManagedControlPlane, arg3 string) error {
	fake.addAWSManagedControlPlaneFinalizerMutex.Lock()
	ret, specificReturn := fake.addAWSManagedControlPlaneFinalizerReturnsOnCall[len(fake.addAWSManagedControlPlaneFinalizerArgsForCall)]
	fake.addAWSManagedControlPlaneFinalizerArgsForCall = append(fake.addAWSManagedControlPlaneFinalizerArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1a.AWSManagedControlPlane
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.AddAWSManagedControlPlaneFinalizerStub
	fakeReturns := fake.addAWSManagedControlPlaneFinalizerReturns
	fake.recordInvocation("AddAWSManagedControlPlaneFinalizer", []interface{}{arg1, arg2, arg3})
	fake.addAWSManagedControlPlaneFinalizerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) AddAWSManagedControlPlaneFinalizerCallCount() int {
	fake.addAWSManagedControlPlaneFinalizerMutex.RLock()
	defer fake.addAWSManagedControlPlaneFinalizerMutex.RUnlock()
	return len(fake.addAWSManagedControlPlaneFinalizerArgsForCall)
}

func (fake *FakeClusterClient) AddAWSManagedControlPlaneFinalizerCalls(stub func(context.Context, *v1beta1a.AWSManagedControlPlane, string) error) {
	fake.addAWSManagedControlPlaneFinalizerMutex.Lock()
	defer fake.addAWSManagedControlPlaneFinalizerMutex.Unlock()
	fake.AddAWSManagedControlPlaneFinalizerStub = stub
}

func (fake *FakeClusterClient) AddAWSManagedControlPlaneFinalizerArgsForCall(i int) (context.Context, *v1beta1a.AWSManagedControlPlane, string) {
	fake.addAWSManagedControlPlaneFinalizerMutex.RLock()
	defer fake.addAWSManagedControlPlaneFinalizerMutex.RUnlock()
	argsForCall := fake.addAWSManagedControlPlaneFinalizerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) AddAWSManagedControlPlaneFinalizerReturns(result1 error) {
	fake.addAWSManagedControlPlaneFinalizerMutex.Lock()
	defer fake.addAWSManagedControlPlaneFinalizerMutex.Unlock()
	fake.AddAWSManagedControlPlaneFinalizerStub = nil
	fake.addAWSManagedControlPlaneFinalizerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) AddAWSManagedControlPlaneFinalizerReturnsOnCall(i int, result1 error) {
	fake.addAWSManagedControlPlaneFinalizerMutex.Lock()
	defer fake.addAWSManagedControlPlaneFinalizerMutex.Unlock()
	fake.AddAWSManagedControlPlaneFinalizerStub = nil
	if fake.addAWSManagedControlPlaneFinalizerReturnsOnCall == nil {
		fake.addAWSManagedControlPlaneFinalizerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.addAWSManagedControlPlaneFinalizerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) AddClusterFinalizer(arg1 context.Context, arg2 *v1beta1b.Cluster, arg3 string) error {
	fake.addClusterFinalizerMutex.Lock()
	ret, specificReturn := fake.addClusterFinalizerReturnsOnCall[len(fake.addClusterFinalizerArgsForCall)]
	fake.addClusterFinalizerArgsForCall = append(fake.addClusterFinalizerArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1b.Cluster
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.AddClusterFinalizerStub
	fakeReturns := fake.addClusterFinalizerReturns
	fake.recordInvocation("AddClusterFinalizer", []interface{}{arg1, arg2, arg3})
	fake.addClusterFinalizerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) AddClusterFinalizerCallCount() int {
	fake.addClusterFinalizerMutex.RLock()
	defer fake.addClusterFinalizerMutex.RUnlock()
	return len(fake.addClusterFinalizerArgsForCall)
}

func (fake *FakeClusterClient) AddClusterFinalizerCalls(stub func(context.Context, *v1beta1b.Cluster, string) error) {
	fake.addClusterFinalizerMutex.Lock()
	defer fake.addClusterFinalizerMutex.Unlock()
	fake.AddClusterFinalizerStub = stub
}

func (fake *FakeClusterClient) AddClusterFinalizerArgsForCall(i int) (context.Context, *v1beta1b.Cluster, string) {
	fake.addClusterFinalizerMutex.RLock()
	defer fake.addClusterFinalizerMutex.RUnlock()
	argsForCall := fake.addClusterFinalizerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) AddClusterFinalizerReturns(result1 error) {
	fake.addClusterFinalizerMutex.Lock()
	defer fake.addClusterFinalizerMutex.Unlock()
	fake.AddClusterFinalizerStub = nil
	fake.addClusterFinalizerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) AddClusterFinalizerReturnsOnCall(i int, result1 error) {
	fake.addClusterFinalizerMutex.Lock()
	defer fake.addClusterFinalizerMutex.Unlock()
	fake.AddClusterFinalizerStub = nil
	if fake.addClusterFinalizerReturnsOnCall == nil {
		fake.addClusterFinalizerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.addClusterFinalizerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) GetAWSCluster(arg1 context.Context, arg2 types.NamespacedName) (*v1beta1.AWSCluster, error) {
	fake.getAWSClusterMutex.Lock()
	ret, specificReturn := fake.getAWSClusterReturnsOnCall[len(fake.getAWSClusterArgsForCall)]
	fake.getAWSClusterArgsForCall = append(fake.getAWSClusterArgsForCall, struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}{arg1, arg2})
	stub := fake.GetAWSClusterStub
	fakeReturns := fake.getAWSClusterReturns
	fake.recordInvocation("GetAWSCluster", []interface{}{arg1, arg2})
	fake.getAWSClusterMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeClusterClient) GetAWSClusterCallCount() int {
	fake.getAWSClusterMutex.RLock()
	defer fake.getAWSClusterMutex.RUnlock()
	return len(fake.getAWSClusterArgsForCall)
}

func (fake *FakeClusterClient) GetAWSClusterCalls(stub func(context.Context, types.NamespacedName) (*v1beta1.AWSCluster, error)) {
	fake.getAWSClusterMutex.Lock()
	defer fake.getAWSClusterMutex.Unlock()
	fake.GetAWSClusterStub = stub
}

func (fake *FakeClusterClient) GetAWSClusterArgsForCall(i int) (context.Context, types.NamespacedName) {
	fake.getAWSClusterMutex.RLock()
	defer fake.getAWSClusterMutex.RUnlock()
	argsForCall := fake.getAWSClusterArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeClusterClient) GetAWSClusterReturns(result1 *v1beta1.AWSCluster, result2 error) {
	fake.getAWSClusterMutex.Lock()
	defer fake.getAWSClusterMutex.Unlock()
	fake.GetAWSClusterStub = nil
	fake.getAWSClusterReturns = struct {
		result1 *v1beta1.AWSCluster
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetAWSClusterReturnsOnCall(i int, result1 *v1beta1.AWSCluster, result2 error) {
	fake.getAWSClusterMutex.Lock()
	defer fake.getAWSClusterMutex.Unlock()
	fake.GetAWSClusterStub = nil
	if fake.getAWSClusterReturnsOnCall == nil {
		fake.getAWSClusterReturnsOnCall = make(map[int]struct {
			result1 *v1beta1.AWSCluster
			result2 error
		})
	}
	fake.getAWSClusterReturnsOnCall[i] = struct {
		result1 *v1beta1.AWSCluster
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetAWSManagedControlPlane(arg1 context.Context, arg2 types.NamespacedName) (*v1beta1a.AWSManagedControlPlane, error) {
	fake.getAWSManagedControlPlaneMutex.Lock()
	ret, specificReturn := fake.getAWSManagedControlPlaneReturnsOnCall[len(fake.getAWSManagedControlPlaneArgsForCall)]
	fake.getAWSManagedControlPlaneArgsForCall = append(fake.getAWSManagedControlPlaneArgsForCall, struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}{arg1, arg2})
	stub := fake.GetAWSManagedControlPlaneStub
	fakeReturns := fake.getAWSManagedControlPlaneReturns
	fake.recordInvocation("GetAWSManagedControlPlane", []interface{}{arg1, arg2})
	fake.getAWSManagedControlPlaneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeClusterClient) GetAWSManagedControlPlaneCallCount() int {
	fake.getAWSManagedControlPlaneMutex.RLock()
	defer fake.getAWSManagedControlPlaneMutex.RUnlock()
	return len(fake.getAWSManagedControlPlaneArgsForCall)
}

func (fake *FakeClusterClient) GetAWSManagedControlPlaneCalls(stub func(context.Context, types.NamespacedName) (*v1beta1a.AWSManagedControlPlane, error)) {
	fake.getAWSManagedControlPlaneMutex.Lock()
	defer fake.getAWSManagedControlPlaneMutex.Unlock()
	fake.GetAWSManagedControlPlaneStub = stub
}

func (fake *FakeClusterClient) GetAWSManagedControlPlaneArgsForCall(i int) (context.Context, types.NamespacedName) {
	fake.getAWSManagedControlPlaneMutex.RLock()
	defer fake.getAWSManagedControlPlaneMutex.RUnlock()
	argsForCall := fake.getAWSManagedControlPlaneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeClusterClient) GetAWSManagedControlPlaneReturns(result1 *v1beta1a.AWSManagedControlPlane, result2 error) {
	fake.getAWSManagedControlPlaneMutex.Lock()
	defer fake.getAWSManagedControlPlaneMutex.Unlock()
	fake.GetAWSManagedControlPlaneStub = nil
	fake.getAWSManagedControlPlaneReturns = struct {
		result1 *v1beta1a.AWSManagedControlPlane
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetAWSManagedControlPlaneReturnsOnCall(i int, result1 *v1beta1a.AWSManagedControlPlane, result2 error) {
	fake.getAWSManagedControlPlaneMutex.Lock()
	defer fake.getAWSManagedControlPlaneMutex.Unlock()
	fake.GetAWSManagedControlPlaneStub = nil
	if fake.getAWSManagedControlPlaneReturnsOnCall == nil {
		fake.getAWSManagedControlPlaneReturnsOnCall = make(map[int]struct {
			result1 *v1beta1a.AWSManagedControlPlane
			result2 error
		})
	}
	fake.getAWSManagedControlPlaneReturnsOnCall[i] = struct {
		result1 *v1beta1a.AWSManagedControlPlane
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetBastionMachine(arg1 context.Context, arg2 string) (*v1beta1b.Machine, error) {
	fake.getBastionMachineMutex.Lock()
	ret, specificReturn := fake.getBastionMachineReturnsOnCall[len(fake.getBastionMachineArgsForCall)]
	fake.getBastionMachineArgsForCall = append(fake.getBastionMachineArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.GetBastionMachineStub
	fakeReturns := fake.getBastionMachineReturns
	fake.recordInvocation("GetBastionMachine", []interface{}{arg1, arg2})
	fake.getBastionMachineMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeClusterClient) GetBastionMachineCallCount() int {
	fake.getBastionMachineMutex.RLock()
	defer fake.getBastionMachineMutex.RUnlock()
	return len(fake.getBastionMachineArgsForCall)
}

func (fake *FakeClusterClient) GetBastionMachineCalls(stub func(context.Context, string) (*v1beta1b.Machine, error)) {
	fake.getBastionMachineMutex.Lock()
	defer fake.getBastionMachineMutex.Unlock()
	fake.GetBastionMachineStub = stub
}

func (fake *FakeClusterClient) GetBastionMachineArgsForCall(i int) (context.Context, string) {
	fake.getBastionMachineMutex.RLock()
	defer fake.getBastionMachineMutex.RUnlock()
	argsForCall := fake.getBastionMachineArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeClusterClient) GetBastionMachineReturns(result1 *v1beta1b.Machine, result2 error) {
	fake.getBastionMachineMutex.Lock()
	defer fake.getBastionMachineMutex.Unlock()
	fake.GetBastionMachineStub = nil
	fake.getBastionMachineReturns = struct {
		result1 *v1beta1b.Machine
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetBastionMachineReturnsOnCall(i int, result1 *v1beta1b.Machine, result2 error) {
	fake.getBastionMachineMutex.Lock()
	defer fake.getBastionMachineMutex.Unlock()
	fake.GetBastionMachineStub = nil
	if fake.getBastionMachineReturnsOnCall == nil {
		fake.getBastionMachineReturnsOnCall = make(map[int]struct {
			result1 *v1beta1b.Machine
			result2 error
		})
	}
	fake.getBastionMachineReturnsOnCall[i] = struct {
		result1 *v1beta1b.Machine
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetCluster(arg1 context.Context, arg2 types.NamespacedName) (*v1beta1b.Cluster, error) {
	fake.getClusterMutex.Lock()
	ret, specificReturn := fake.getClusterReturnsOnCall[len(fake.getClusterArgsForCall)]
	fake.getClusterArgsForCall = append(fake.getClusterArgsForCall, struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}{arg1, arg2})
	stub := fake.GetClusterStub
	fakeReturns := fake.getClusterReturns
	fake.recordInvocation("GetCluster", []interface{}{arg1, arg2})
	fake.getClusterMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeClusterClient) GetClusterCallCount() int {
	fake.getClusterMutex.RLock()
	defer fake.getClusterMutex.RUnlock()
	return len(fake.getClusterArgsForCall)
}

func (fake *FakeClusterClient) GetClusterCalls(stub func(context.Context, types.NamespacedName) (*v1beta1b.Cluster, error)) {
	fake.getClusterMutex.Lock()
	defer fake.getClusterMutex.Unlock()
	fake.GetClusterStub = stub
}

func (fake *FakeClusterClient) GetClusterArgsForCall(i int) (context.Context, types.NamespacedName) {
	fake.getClusterMutex.RLock()
	defer fake.getClusterMutex.RUnlock()
	argsForCall := fake.getClusterArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeClusterClient) GetClusterReturns(result1 *v1beta1b.Cluster, result2 error) {
	fake.getClusterMutex.Lock()
	defer fake.getClusterMutex.Unlock()
	fake.GetClusterStub = nil
	fake.getClusterReturns = struct {
		result1 *v1beta1b.Cluster
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetClusterReturnsOnCall(i int, result1 *v1beta1b.Cluster, result2 error) {
	fake.getClusterMutex.Lock()
	defer fake.getClusterMutex.Unlock()
	fake.GetClusterStub = nil
	if fake.getClusterReturnsOnCall == nil {
		fake.getClusterReturnsOnCall = make(map[int]struct {
			result1 *v1beta1b.Cluster
			result2 error
		})
	}
	fake.getClusterReturnsOnCall[i] = struct {
		result1 *v1beta1b.Cluster
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetIdentity(arg1 context.Context, arg2 *v1beta1.AWSIdentityReference) (*v1beta1.AWSClusterRoleIdentity, error) {
	fake.getIdentityMutex.Lock()
	ret, specificReturn := fake.getIdentityReturnsOnCall[len(fake.getIdentityArgsForCall)]
	fake.getIdentityArgsForCall = append(fake.getIdentityArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1.AWSIdentityReference
	}{arg1, arg2})
	stub := fake.GetIdentityStub
	fakeReturns := fake.getIdentityReturns
	fake.recordInvocation("GetIdentity", []interface{}{arg1, arg2})
	fake.getIdentityMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeClusterClient) GetIdentityCallCount() int {
	fake.getIdentityMutex.RLock()
	defer fake.getIdentityMutex.RUnlock()
	return len(fake.getIdentityArgsForCall)
}

func (fake *FakeClusterClient) GetIdentityCalls(stub func(context.Context, *v1beta1.AWSIdentityReference) (*v1beta1.AWSClusterRoleIdentity, error)) {
	fake.getIdentityMutex.Lock()
	defer fake.getIdentityMutex.Unlock()
	fake.GetIdentityStub = stub
}

func (fake *FakeClusterClient) GetIdentityArgsForCall(i int) (context.Context, *v1beta1.AWSIdentityReference) {
	fake.getIdentityMutex.RLock()
	defer fake.getIdentityMutex.RUnlock()
	argsForCall := fake.getIdentityArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeClusterClient) GetIdentityReturns(result1 *v1beta1.AWSClusterRoleIdentity, result2 error) {
	fake.getIdentityMutex.Lock()
	defer fake.getIdentityMutex.Unlock()
	fake.GetIdentityStub = nil
	fake.getIdentityReturns = struct {
		result1 *v1beta1.AWSClusterRoleIdentity
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) GetIdentityReturnsOnCall(i int, result1 *v1beta1.AWSClusterRoleIdentity, result2 error) {
	fake.getIdentityMutex.Lock()
	defer fake.getIdentityMutex.Unlock()
	fake.GetIdentityStub = nil
	if fake.getIdentityReturnsOnCall == nil {
		fake.getIdentityReturnsOnCall = make(map[int]struct {
			result1 *v1beta1.AWSClusterRoleIdentity
			result2 error
		})
	}
	fake.getIdentityReturnsOnCall[i] = struct {
		result1 *v1beta1.AWSClusterRoleIdentity
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterClient) MarkConditionTrue(arg1 context.Context, arg2 *v1beta1b.Cluster, arg3 v1beta1b.ConditionType) error {
	fake.markConditionTrueMutex.Lock()
	ret, specificReturn := fake.markConditionTrueReturnsOnCall[len(fake.markConditionTrueArgsForCall)]
	fake.markConditionTrueArgsForCall = append(fake.markConditionTrueArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1b.Cluster
		arg3 v1beta1b.ConditionType
	}{arg1, arg2, arg3})
	stub := fake.MarkConditionTrueStub
	fakeReturns := fake.markConditionTrueReturns
	fake.recordInvocation("MarkConditionTrue", []interface{}{arg1, arg2, arg3})
	fake.markConditionTrueMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) MarkConditionTrueCallCount() int {
	fake.markConditionTrueMutex.RLock()
	defer fake.markConditionTrueMutex.RUnlock()
	return len(fake.markConditionTrueArgsForCall)
}

func (fake *FakeClusterClient) MarkConditionTrueCalls(stub func(context.Context, *v1beta1b.Cluster, v1beta1b.ConditionType) error) {
	fake.markConditionTrueMutex.Lock()
	defer fake.markConditionTrueMutex.Unlock()
	fake.MarkConditionTrueStub = stub
}

func (fake *FakeClusterClient) MarkConditionTrueArgsForCall(i int) (context.Context, *v1beta1b.Cluster, v1beta1b.ConditionType) {
	fake.markConditionTrueMutex.RLock()
	defer fake.markConditionTrueMutex.RUnlock()
	argsForCall := fake.markConditionTrueArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) MarkConditionTrueReturns(result1 error) {
	fake.markConditionTrueMutex.Lock()
	defer fake.markConditionTrueMutex.Unlock()
	fake.MarkConditionTrueStub = nil
	fake.markConditionTrueReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) MarkConditionTrueReturnsOnCall(i int, result1 error) {
	fake.markConditionTrueMutex.Lock()
	defer fake.markConditionTrueMutex.Unlock()
	fake.MarkConditionTrueStub = nil
	if fake.markConditionTrueReturnsOnCall == nil {
		fake.markConditionTrueReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.markConditionTrueReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) RemoveAWSClusterFinalizer(arg1 context.Context, arg2 *v1beta1.AWSCluster, arg3 string) error {
	fake.removeAWSClusterFinalizerMutex.Lock()
	ret, specificReturn := fake.removeAWSClusterFinalizerReturnsOnCall[len(fake.removeAWSClusterFinalizerArgsForCall)]
	fake.removeAWSClusterFinalizerArgsForCall = append(fake.removeAWSClusterFinalizerArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1.AWSCluster
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.RemoveAWSClusterFinalizerStub
	fakeReturns := fake.removeAWSClusterFinalizerReturns
	fake.recordInvocation("RemoveAWSClusterFinalizer", []interface{}{arg1, arg2, arg3})
	fake.removeAWSClusterFinalizerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) RemoveAWSClusterFinalizerCallCount() int {
	fake.removeAWSClusterFinalizerMutex.RLock()
	defer fake.removeAWSClusterFinalizerMutex.RUnlock()
	return len(fake.removeAWSClusterFinalizerArgsForCall)
}

func (fake *FakeClusterClient) RemoveAWSClusterFinalizerCalls(stub func(context.Context, *v1beta1.AWSCluster, string) error) {
	fake.removeAWSClusterFinalizerMutex.Lock()
	defer fake.removeAWSClusterFinalizerMutex.Unlock()
	fake.RemoveAWSClusterFinalizerStub = stub
}

func (fake *FakeClusterClient) RemoveAWSClusterFinalizerArgsForCall(i int) (context.Context, *v1beta1.AWSCluster, string) {
	fake.removeAWSClusterFinalizerMutex.RLock()
	defer fake.removeAWSClusterFinalizerMutex.RUnlock()
	argsForCall := fake.removeAWSClusterFinalizerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) RemoveAWSClusterFinalizerReturns(result1 error) {
	fake.removeAWSClusterFinalizerMutex.Lock()
	defer fake.removeAWSClusterFinalizerMutex.Unlock()
	fake.RemoveAWSClusterFinalizerStub = nil
	fake.removeAWSClusterFinalizerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) RemoveAWSClusterFinalizerReturnsOnCall(i int, result1 error) {
	fake.removeAWSClusterFinalizerMutex.Lock()
	defer fake.removeAWSClusterFinalizerMutex.Unlock()
	fake.RemoveAWSClusterFinalizerStub = nil
	if fake.removeAWSClusterFinalizerReturnsOnCall == nil {
		fake.removeAWSClusterFinalizerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.removeAWSClusterFinalizerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) RemoveAWSManagedControlPlaneFinalizer(arg1 context.Context, arg2 *v1beta1a.AWSManagedControlPlane, arg3 string) error {
	fake.removeAWSManagedControlPlaneFinalizerMutex.Lock()
	ret, specificReturn := fake.removeAWSManagedControlPlaneFinalizerReturnsOnCall[len(fake.removeAWSManagedControlPlaneFinalizerArgsForCall)]
	fake.removeAWSManagedControlPlaneFinalizerArgsForCall = append(fake.removeAWSManagedControlPlaneFinalizerArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1a.AWSManagedControlPlane
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.RemoveAWSManagedControlPlaneFinalizerStub
	fakeReturns := fake.removeAWSManagedControlPlaneFinalizerReturns
	fake.recordInvocation("RemoveAWSManagedControlPlaneFinalizer", []interface{}{arg1, arg2, arg3})
	fake.removeAWSManagedControlPlaneFinalizerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) RemoveAWSManagedControlPlaneFinalizerCallCount() int {
	fake.removeAWSManagedControlPlaneFinalizerMutex.RLock()
	defer fake.removeAWSManagedControlPlaneFinalizerMutex.RUnlock()
	return len(fake.removeAWSManagedControlPlaneFinalizerArgsForCall)
}

func (fake *FakeClusterClient) RemoveAWSManagedControlPlaneFinalizerCalls(stub func(context.Context, *v1beta1a.AWSManagedControlPlane, string) error) {
	fake.removeAWSManagedControlPlaneFinalizerMutex.Lock()
	defer fake.removeAWSManagedControlPlaneFinalizerMutex.Unlock()
	fake.RemoveAWSManagedControlPlaneFinalizerStub = stub
}

func (fake *FakeClusterClient) RemoveAWSManagedControlPlaneFinalizerArgsForCall(i int) (context.Context, *v1beta1a.AWSManagedControlPlane, string) {
	fake.removeAWSManagedControlPlaneFinalizerMutex.RLock()
	defer fake.removeAWSManagedControlPlaneFinalizerMutex.RUnlock()
	argsForCall := fake.removeAWSManagedControlPlaneFinalizerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) RemoveAWSManagedControlPlaneFinalizerReturns(result1 error) {
	fake.removeAWSManagedControlPlaneFinalizerMutex.Lock()
	defer fake.removeAWSManagedControlPlaneFinalizerMutex.Unlock()
	fake.RemoveAWSManagedControlPlaneFinalizerStub = nil
	fake.removeAWSManagedControlPlaneFinalizerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) RemoveAWSManagedControlPlaneFinalizerReturnsOnCall(i int, result1 error) {
	fake.removeAWSManagedControlPlaneFinalizerMutex.Lock()
	defer fake.removeAWSManagedControlPlaneFinalizerMutex.Unlock()
	fake.RemoveAWSManagedControlPlaneFinalizerStub = nil
	if fake.removeAWSManagedControlPlaneFinalizerReturnsOnCall == nil {
		fake.removeAWSManagedControlPlaneFinalizerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.removeAWSManagedControlPlaneFinalizerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) RemoveClusterFinalizer(arg1 context.Context, arg2 *v1beta1b.Cluster, arg3 string) error {
	fake.removeClusterFinalizerMutex.Lock()
	ret, specificReturn := fake.removeClusterFinalizerReturnsOnCall[len(fake.removeClusterFinalizerArgsForCall)]
	fake.removeClusterFinalizerArgsForCall = append(fake.removeClusterFinalizerArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1b.Cluster
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.RemoveClusterFinalizerStub
	fakeReturns := fake.removeClusterFinalizerReturns
	fake.recordInvocation("RemoveClusterFinalizer", []interface{}{arg1, arg2, arg3})
	fake.removeClusterFinalizerMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) RemoveClusterFinalizerCallCount() int {
	fake.removeClusterFinalizerMutex.RLock()
	defer fake.removeClusterFinalizerMutex.RUnlock()
	return len(fake.removeClusterFinalizerArgsForCall)
}

func (fake *FakeClusterClient) RemoveClusterFinalizerCalls(stub func(context.Context, *v1beta1b.Cluster, string) error) {
	fake.removeClusterFinalizerMutex.Lock()
	defer fake.removeClusterFinalizerMutex.Unlock()
	fake.RemoveClusterFinalizerStub = stub
}

func (fake *FakeClusterClient) RemoveClusterFinalizerArgsForCall(i int) (context.Context, *v1beta1b.Cluster, string) {
	fake.removeClusterFinalizerMutex.RLock()
	defer fake.removeClusterFinalizerMutex.RUnlock()
	argsForCall := fake.removeClusterFinalizerArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) RemoveClusterFinalizerReturns(result1 error) {
	fake.removeClusterFinalizerMutex.Lock()
	defer fake.removeClusterFinalizerMutex.Unlock()
	fake.RemoveClusterFinalizerStub = nil
	fake.removeClusterFinalizerReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) RemoveClusterFinalizerReturnsOnCall(i int, result1 error) {
	fake.removeClusterFinalizerMutex.Lock()
	defer fake.removeClusterFinalizerMutex.Unlock()
	fake.RemoveClusterFinalizerStub = nil
	if fake.removeClusterFinalizerReturnsOnCall == nil {
		fake.removeClusterFinalizerReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.removeClusterFinalizerReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) Unpause(arg1 context.Context, arg2 *v1beta1.AWSCluster, arg3 *v1beta1b.Cluster) error {
	fake.unpauseMutex.Lock()
	ret, specificReturn := fake.unpauseReturnsOnCall[len(fake.unpauseArgsForCall)]
	fake.unpauseArgsForCall = append(fake.unpauseArgsForCall, struct {
		arg1 context.Context
		arg2 *v1beta1.AWSCluster
		arg3 *v1beta1b.Cluster
	}{arg1, arg2, arg3})
	stub := fake.UnpauseStub
	fakeReturns := fake.unpauseReturns
	fake.recordInvocation("Unpause", []interface{}{arg1, arg2, arg3})
	fake.unpauseMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeClusterClient) UnpauseCallCount() int {
	fake.unpauseMutex.RLock()
	defer fake.unpauseMutex.RUnlock()
	return len(fake.unpauseArgsForCall)
}

func (fake *FakeClusterClient) UnpauseCalls(stub func(context.Context, *v1beta1.AWSCluster, *v1beta1b.Cluster) error) {
	fake.unpauseMutex.Lock()
	defer fake.unpauseMutex.Unlock()
	fake.UnpauseStub = stub
}

func (fake *FakeClusterClient) UnpauseArgsForCall(i int) (context.Context, *v1beta1.AWSCluster, *v1beta1b.Cluster) {
	fake.unpauseMutex.RLock()
	defer fake.unpauseMutex.RUnlock()
	argsForCall := fake.unpauseArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterClient) UnpauseReturns(result1 error) {
	fake.unpauseMutex.Lock()
	defer fake.unpauseMutex.Unlock()
	fake.UnpauseStub = nil
	fake.unpauseReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) UnpauseReturnsOnCall(i int, result1 error) {
	fake.unpauseMutex.Lock()
	defer fake.unpauseMutex.Unlock()
	fake.UnpauseStub = nil
	if fake.unpauseReturnsOnCall == nil {
		fake.unpauseReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.unpauseReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeClusterClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.addAWSClusterFinalizerMutex.RLock()
	defer fake.addAWSClusterFinalizerMutex.RUnlock()
	fake.addAWSManagedControlPlaneFinalizerMutex.RLock()
	defer fake.addAWSManagedControlPlaneFinalizerMutex.RUnlock()
	fake.addClusterFinalizerMutex.RLock()
	defer fake.addClusterFinalizerMutex.RUnlock()
	fake.getAWSClusterMutex.RLock()
	defer fake.getAWSClusterMutex.RUnlock()
	fake.getAWSManagedControlPlaneMutex.RLock()
	defer fake.getAWSManagedControlPlaneMutex.RUnlock()
	fake.getBastionMachineMutex.RLock()
	defer fake.getBastionMachineMutex.RUnlock()
	fake.getClusterMutex.RLock()
	defer fake.getClusterMutex.RUnlock()
	fake.getIdentityMutex.RLock()
	defer fake.getIdentityMutex.RUnlock()
	fake.markConditionTrueMutex.RLock()
	defer fake.markConditionTrueMutex.RUnlock()
	fake.removeAWSClusterFinalizerMutex.RLock()
	defer fake.removeAWSClusterFinalizerMutex.RUnlock()
	fake.removeAWSManagedControlPlaneFinalizerMutex.RLock()
	defer fake.removeAWSManagedControlPlaneFinalizerMutex.RUnlock()
	fake.removeClusterFinalizerMutex.RLock()
	defer fake.removeClusterFinalizerMutex.RUnlock()
	fake.unpauseMutex.RLock()
	defer fake.unpauseMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeClusterClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ controllers.ClusterClient = new(FakeClusterClient)
