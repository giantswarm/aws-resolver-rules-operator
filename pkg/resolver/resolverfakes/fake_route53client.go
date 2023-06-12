// Code generated by counterfeiter. DO NOT EDIT.
package resolverfakes

import (
	"context"
	"sync"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/go-logr/logr"
)

type FakeRoute53Client struct {
	AddDelegationToParentZoneStub        func(context.Context, logr.Logger, string, string) error
	addDelegationToParentZoneMutex       sync.RWMutex
	addDelegationToParentZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 string
	}
	addDelegationToParentZoneReturns struct {
		result1 error
	}
	addDelegationToParentZoneReturnsOnCall map[int]struct {
		result1 error
	}
	CreatePrivateHostedZoneStub        func(context.Context, logr.Logger, string, string, string, map[string]string, []string) (string, error)
	createPrivateHostedZoneMutex       sync.RWMutex
	createPrivateHostedZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 string
		arg5 string
		arg6 map[string]string
		arg7 []string
	}
	createPrivateHostedZoneReturns struct {
		result1 string
		result2 error
	}
	createPrivateHostedZoneReturnsOnCall map[int]struct {
		result1 string
		result2 error
	}
	CreatePublicHostedZoneStub        func(context.Context, logr.Logger, string, map[string]string) (string, error)
	createPublicHostedZoneMutex       sync.RWMutex
	createPublicHostedZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 map[string]string
	}
	createPublicHostedZoneReturns struct {
		result1 string
		result2 error
	}
	createPublicHostedZoneReturnsOnCall map[int]struct {
		result1 string
		result2 error
	}
	DeleteDelegationFromParentZoneStub        func(context.Context, logr.Logger, string, string) error
	deleteDelegationFromParentZoneMutex       sync.RWMutex
	deleteDelegationFromParentZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 string
	}
	deleteDelegationFromParentZoneReturns struct {
		result1 error
	}
	deleteDelegationFromParentZoneReturnsOnCall map[int]struct {
		result1 error
	}
	DeleteHostedZoneStub        func(context.Context, logr.Logger, string) error
	deleteHostedZoneMutex       sync.RWMutex
	deleteHostedZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
	}
	deleteHostedZoneReturns struct {
		result1 error
	}
	deleteHostedZoneReturnsOnCall map[int]struct {
		result1 error
	}
	GetHostedZoneIdByNameStub        func(context.Context, logr.Logger, string) (string, error)
	getHostedZoneIdByNameMutex       sync.RWMutex
	getHostedZoneIdByNameArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
	}
	getHostedZoneIdByNameReturns struct {
		result1 string
		result2 error
	}
	getHostedZoneIdByNameReturnsOnCall map[int]struct {
		result1 string
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRoute53Client) AddDelegationToParentZone(arg1 context.Context, arg2 logr.Logger, arg3 string, arg4 string) error {
	fake.addDelegationToParentZoneMutex.Lock()
	ret, specificReturn := fake.addDelegationToParentZoneReturnsOnCall[len(fake.addDelegationToParentZoneArgsForCall)]
	fake.addDelegationToParentZoneArgsForCall = append(fake.addDelegationToParentZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.AddDelegationToParentZoneStub
	fakeReturns := fake.addDelegationToParentZoneReturns
	fake.recordInvocation("AddDelegationToParentZone", []interface{}{arg1, arg2, arg3, arg4})
	fake.addDelegationToParentZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeRoute53Client) AddDelegationToParentZoneCallCount() int {
	fake.addDelegationToParentZoneMutex.RLock()
	defer fake.addDelegationToParentZoneMutex.RUnlock()
	return len(fake.addDelegationToParentZoneArgsForCall)
}

func (fake *FakeRoute53Client) AddDelegationToParentZoneCalls(stub func(context.Context, logr.Logger, string, string) error) {
	fake.addDelegationToParentZoneMutex.Lock()
	defer fake.addDelegationToParentZoneMutex.Unlock()
	fake.AddDelegationToParentZoneStub = stub
}

func (fake *FakeRoute53Client) AddDelegationToParentZoneArgsForCall(i int) (context.Context, logr.Logger, string, string) {
	fake.addDelegationToParentZoneMutex.RLock()
	defer fake.addDelegationToParentZoneMutex.RUnlock()
	argsForCall := fake.addDelegationToParentZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeRoute53Client) AddDelegationToParentZoneReturns(result1 error) {
	fake.addDelegationToParentZoneMutex.Lock()
	defer fake.addDelegationToParentZoneMutex.Unlock()
	fake.AddDelegationToParentZoneStub = nil
	fake.addDelegationToParentZoneReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) AddDelegationToParentZoneReturnsOnCall(i int, result1 error) {
	fake.addDelegationToParentZoneMutex.Lock()
	defer fake.addDelegationToParentZoneMutex.Unlock()
	fake.AddDelegationToParentZoneStub = nil
	if fake.addDelegationToParentZoneReturnsOnCall == nil {
		fake.addDelegationToParentZoneReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.addDelegationToParentZoneReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) CreatePrivateHostedZone(arg1 context.Context, arg2 logr.Logger, arg3 string, arg4 string, arg5 string, arg6 map[string]string, arg7 []string) (string, error) {
	var arg7Copy []string
	if arg7 != nil {
		arg7Copy = make([]string, len(arg7))
		copy(arg7Copy, arg7)
	}
	fake.createPrivateHostedZoneMutex.Lock()
	ret, specificReturn := fake.createPrivateHostedZoneReturnsOnCall[len(fake.createPrivateHostedZoneArgsForCall)]
	fake.createPrivateHostedZoneArgsForCall = append(fake.createPrivateHostedZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 string
		arg5 string
		arg6 map[string]string
		arg7 []string
	}{arg1, arg2, arg3, arg4, arg5, arg6, arg7Copy})
	stub := fake.CreatePrivateHostedZoneStub
	fakeReturns := fake.createPrivateHostedZoneReturns
	fake.recordInvocation("CreatePrivateHostedZone", []interface{}{arg1, arg2, arg3, arg4, arg5, arg6, arg7Copy})
	fake.createPrivateHostedZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4, arg5, arg6, arg7)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeRoute53Client) CreatePrivateHostedZoneCallCount() int {
	fake.createPrivateHostedZoneMutex.RLock()
	defer fake.createPrivateHostedZoneMutex.RUnlock()
	return len(fake.createPrivateHostedZoneArgsForCall)
}

func (fake *FakeRoute53Client) CreatePrivateHostedZoneCalls(stub func(context.Context, logr.Logger, string, string, string, map[string]string, []string) (string, error)) {
	fake.createPrivateHostedZoneMutex.Lock()
	defer fake.createPrivateHostedZoneMutex.Unlock()
	fake.CreatePrivateHostedZoneStub = stub
}

func (fake *FakeRoute53Client) CreatePrivateHostedZoneArgsForCall(i int) (context.Context, logr.Logger, string, string, string, map[string]string, []string) {
	fake.createPrivateHostedZoneMutex.RLock()
	defer fake.createPrivateHostedZoneMutex.RUnlock()
	argsForCall := fake.createPrivateHostedZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4, argsForCall.arg5, argsForCall.arg6, argsForCall.arg7
}

func (fake *FakeRoute53Client) CreatePrivateHostedZoneReturns(result1 string, result2 error) {
	fake.createPrivateHostedZoneMutex.Lock()
	defer fake.createPrivateHostedZoneMutex.Unlock()
	fake.CreatePrivateHostedZoneStub = nil
	fake.createPrivateHostedZoneReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeRoute53Client) CreatePrivateHostedZoneReturnsOnCall(i int, result1 string, result2 error) {
	fake.createPrivateHostedZoneMutex.Lock()
	defer fake.createPrivateHostedZoneMutex.Unlock()
	fake.CreatePrivateHostedZoneStub = nil
	if fake.createPrivateHostedZoneReturnsOnCall == nil {
		fake.createPrivateHostedZoneReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.createPrivateHostedZoneReturnsOnCall[i] = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeRoute53Client) CreatePublicHostedZone(arg1 context.Context, arg2 logr.Logger, arg3 string, arg4 map[string]string) (string, error) {
	fake.createPublicHostedZoneMutex.Lock()
	ret, specificReturn := fake.createPublicHostedZoneReturnsOnCall[len(fake.createPublicHostedZoneArgsForCall)]
	fake.createPublicHostedZoneArgsForCall = append(fake.createPublicHostedZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 map[string]string
	}{arg1, arg2, arg3, arg4})
	stub := fake.CreatePublicHostedZoneStub
	fakeReturns := fake.createPublicHostedZoneReturns
	fake.recordInvocation("CreatePublicHostedZone", []interface{}{arg1, arg2, arg3, arg4})
	fake.createPublicHostedZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeRoute53Client) CreatePublicHostedZoneCallCount() int {
	fake.createPublicHostedZoneMutex.RLock()
	defer fake.createPublicHostedZoneMutex.RUnlock()
	return len(fake.createPublicHostedZoneArgsForCall)
}

func (fake *FakeRoute53Client) CreatePublicHostedZoneCalls(stub func(context.Context, logr.Logger, string, map[string]string) (string, error)) {
	fake.createPublicHostedZoneMutex.Lock()
	defer fake.createPublicHostedZoneMutex.Unlock()
	fake.CreatePublicHostedZoneStub = stub
}

func (fake *FakeRoute53Client) CreatePublicHostedZoneArgsForCall(i int) (context.Context, logr.Logger, string, map[string]string) {
	fake.createPublicHostedZoneMutex.RLock()
	defer fake.createPublicHostedZoneMutex.RUnlock()
	argsForCall := fake.createPublicHostedZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeRoute53Client) CreatePublicHostedZoneReturns(result1 string, result2 error) {
	fake.createPublicHostedZoneMutex.Lock()
	defer fake.createPublicHostedZoneMutex.Unlock()
	fake.CreatePublicHostedZoneStub = nil
	fake.createPublicHostedZoneReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeRoute53Client) CreatePublicHostedZoneReturnsOnCall(i int, result1 string, result2 error) {
	fake.createPublicHostedZoneMutex.Lock()
	defer fake.createPublicHostedZoneMutex.Unlock()
	fake.CreatePublicHostedZoneStub = nil
	if fake.createPublicHostedZoneReturnsOnCall == nil {
		fake.createPublicHostedZoneReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.createPublicHostedZoneReturnsOnCall[i] = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeRoute53Client) DeleteDelegationFromParentZone(arg1 context.Context, arg2 logr.Logger, arg3 string, arg4 string) error {
	fake.deleteDelegationFromParentZoneMutex.Lock()
	ret, specificReturn := fake.deleteDelegationFromParentZoneReturnsOnCall[len(fake.deleteDelegationFromParentZoneArgsForCall)]
	fake.deleteDelegationFromParentZoneArgsForCall = append(fake.deleteDelegationFromParentZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.DeleteDelegationFromParentZoneStub
	fakeReturns := fake.deleteDelegationFromParentZoneReturns
	fake.recordInvocation("DeleteDelegationFromParentZone", []interface{}{arg1, arg2, arg3, arg4})
	fake.deleteDelegationFromParentZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeRoute53Client) DeleteDelegationFromParentZoneCallCount() int {
	fake.deleteDelegationFromParentZoneMutex.RLock()
	defer fake.deleteDelegationFromParentZoneMutex.RUnlock()
	return len(fake.deleteDelegationFromParentZoneArgsForCall)
}

func (fake *FakeRoute53Client) DeleteDelegationFromParentZoneCalls(stub func(context.Context, logr.Logger, string, string) error) {
	fake.deleteDelegationFromParentZoneMutex.Lock()
	defer fake.deleteDelegationFromParentZoneMutex.Unlock()
	fake.DeleteDelegationFromParentZoneStub = stub
}

func (fake *FakeRoute53Client) DeleteDelegationFromParentZoneArgsForCall(i int) (context.Context, logr.Logger, string, string) {
	fake.deleteDelegationFromParentZoneMutex.RLock()
	defer fake.deleteDelegationFromParentZoneMutex.RUnlock()
	argsForCall := fake.deleteDelegationFromParentZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeRoute53Client) DeleteDelegationFromParentZoneReturns(result1 error) {
	fake.deleteDelegationFromParentZoneMutex.Lock()
	defer fake.deleteDelegationFromParentZoneMutex.Unlock()
	fake.DeleteDelegationFromParentZoneStub = nil
	fake.deleteDelegationFromParentZoneReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) DeleteDelegationFromParentZoneReturnsOnCall(i int, result1 error) {
	fake.deleteDelegationFromParentZoneMutex.Lock()
	defer fake.deleteDelegationFromParentZoneMutex.Unlock()
	fake.DeleteDelegationFromParentZoneStub = nil
	if fake.deleteDelegationFromParentZoneReturnsOnCall == nil {
		fake.deleteDelegationFromParentZoneReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteDelegationFromParentZoneReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) DeleteHostedZone(arg1 context.Context, arg2 logr.Logger, arg3 string) error {
	fake.deleteHostedZoneMutex.Lock()
	ret, specificReturn := fake.deleteHostedZoneReturnsOnCall[len(fake.deleteHostedZoneArgsForCall)]
	fake.deleteHostedZoneArgsForCall = append(fake.deleteHostedZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.DeleteHostedZoneStub
	fakeReturns := fake.deleteHostedZoneReturns
	fake.recordInvocation("DeleteHostedZone", []interface{}{arg1, arg2, arg3})
	fake.deleteHostedZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeRoute53Client) DeleteHostedZoneCallCount() int {
	fake.deleteHostedZoneMutex.RLock()
	defer fake.deleteHostedZoneMutex.RUnlock()
	return len(fake.deleteHostedZoneArgsForCall)
}

func (fake *FakeRoute53Client) DeleteHostedZoneCalls(stub func(context.Context, logr.Logger, string) error) {
	fake.deleteHostedZoneMutex.Lock()
	defer fake.deleteHostedZoneMutex.Unlock()
	fake.DeleteHostedZoneStub = stub
}

func (fake *FakeRoute53Client) DeleteHostedZoneArgsForCall(i int) (context.Context, logr.Logger, string) {
	fake.deleteHostedZoneMutex.RLock()
	defer fake.deleteHostedZoneMutex.RUnlock()
	argsForCall := fake.deleteHostedZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeRoute53Client) DeleteHostedZoneReturns(result1 error) {
	fake.deleteHostedZoneMutex.Lock()
	defer fake.deleteHostedZoneMutex.Unlock()
	fake.DeleteHostedZoneStub = nil
	fake.deleteHostedZoneReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) DeleteHostedZoneReturnsOnCall(i int, result1 error) {
	fake.deleteHostedZoneMutex.Lock()
	defer fake.deleteHostedZoneMutex.Unlock()
	fake.DeleteHostedZoneStub = nil
	if fake.deleteHostedZoneReturnsOnCall == nil {
		fake.deleteHostedZoneReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteHostedZoneReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) GetHostedZoneIdByName(arg1 context.Context, arg2 logr.Logger, arg3 string) (string, error) {
	fake.getHostedZoneIdByNameMutex.Lock()
	ret, specificReturn := fake.getHostedZoneIdByNameReturnsOnCall[len(fake.getHostedZoneIdByNameArgsForCall)]
	fake.getHostedZoneIdByNameArgsForCall = append(fake.getHostedZoneIdByNameArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.GetHostedZoneIdByNameStub
	fakeReturns := fake.getHostedZoneIdByNameReturns
	fake.recordInvocation("GetHostedZoneIdByName", []interface{}{arg1, arg2, arg3})
	fake.getHostedZoneIdByNameMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeRoute53Client) GetHostedZoneIdByNameCallCount() int {
	fake.getHostedZoneIdByNameMutex.RLock()
	defer fake.getHostedZoneIdByNameMutex.RUnlock()
	return len(fake.getHostedZoneIdByNameArgsForCall)
}

func (fake *FakeRoute53Client) GetHostedZoneIdByNameCalls(stub func(context.Context, logr.Logger, string) (string, error)) {
	fake.getHostedZoneIdByNameMutex.Lock()
	defer fake.getHostedZoneIdByNameMutex.Unlock()
	fake.GetHostedZoneIdByNameStub = stub
}

func (fake *FakeRoute53Client) GetHostedZoneIdByNameArgsForCall(i int) (context.Context, logr.Logger, string) {
	fake.getHostedZoneIdByNameMutex.RLock()
	defer fake.getHostedZoneIdByNameMutex.RUnlock()
	argsForCall := fake.getHostedZoneIdByNameArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeRoute53Client) GetHostedZoneIdByNameReturns(result1 string, result2 error) {
	fake.getHostedZoneIdByNameMutex.Lock()
	defer fake.getHostedZoneIdByNameMutex.Unlock()
	fake.GetHostedZoneIdByNameStub = nil
	fake.getHostedZoneIdByNameReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeRoute53Client) GetHostedZoneIdByNameReturnsOnCall(i int, result1 string, result2 error) {
	fake.getHostedZoneIdByNameMutex.Lock()
	defer fake.getHostedZoneIdByNameMutex.Unlock()
	fake.GetHostedZoneIdByNameStub = nil
	if fake.getHostedZoneIdByNameReturnsOnCall == nil {
		fake.getHostedZoneIdByNameReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.getHostedZoneIdByNameReturnsOnCall[i] = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeRoute53Client) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.addDelegationToParentZoneMutex.RLock()
	defer fake.addDelegationToParentZoneMutex.RUnlock()
	fake.createPrivateHostedZoneMutex.RLock()
	defer fake.createPrivateHostedZoneMutex.RUnlock()
	fake.createPublicHostedZoneMutex.RLock()
	defer fake.createPublicHostedZoneMutex.RUnlock()
	fake.deleteDelegationFromParentZoneMutex.RLock()
	defer fake.deleteDelegationFromParentZoneMutex.RUnlock()
	fake.deleteHostedZoneMutex.RLock()
	defer fake.deleteHostedZoneMutex.RUnlock()
	fake.getHostedZoneIdByNameMutex.RLock()
	defer fake.getHostedZoneIdByNameMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeRoute53Client) recordInvocation(key string, args []interface{}) {
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

var _ resolver.Route53Client = new(FakeRoute53Client)
