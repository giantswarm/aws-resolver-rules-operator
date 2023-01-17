// Code generated by counterfeiter. DO NOT EDIT.
package resolverfakes

import (
	"context"
	"sync"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
	"github.com/go-logr/logr"
)

type FakeResolverClient struct {
	AssociateResolverRuleWithContextStub        func(context.Context, string, string, string) error
	associateResolverRuleWithContextMutex       sync.RWMutex
	associateResolverRuleWithContextArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 string
	}
	associateResolverRuleWithContextReturns struct {
		result1 error
	}
	associateResolverRuleWithContextReturnsOnCall map[int]struct {
		result1 error
	}
	CreateResolverRuleStub        func(context.Context, logr.Logger, resolver.Cluster, string, string, string) (string, string, error)
	createResolverRuleMutex       sync.RWMutex
	createResolverRuleArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 resolver.Cluster
		arg4 string
		arg5 string
		arg6 string
	}
	createResolverRuleReturns struct {
		result1 string
		result2 string
		result3 error
	}
	createResolverRuleReturnsOnCall map[int]struct {
		result1 string
		result2 string
		result3 error
	}
	DeleteResolverRuleStub        func(context.Context, logr.Logger, resolver.Cluster, string) error
	deleteResolverRuleMutex       sync.RWMutex
	deleteResolverRuleArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 resolver.Cluster
		arg4 string
	}
	deleteResolverRuleReturns struct {
		result1 error
	}
	deleteResolverRuleReturnsOnCall map[int]struct {
		result1 error
	}
	DisassociateResolverRuleWithContextStub        func(context.Context, string, string) error
	disassociateResolverRuleWithContextMutex       sync.RWMutex
	disassociateResolverRuleWithContextArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}
	disassociateResolverRuleWithContextReturns struct {
		result1 error
	}
	disassociateResolverRuleWithContextReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeResolverClient) AssociateResolverRuleWithContext(arg1 context.Context, arg2 string, arg3 string, arg4 string) error {
	fake.associateResolverRuleWithContextMutex.Lock()
	ret, specificReturn := fake.associateResolverRuleWithContextReturnsOnCall[len(fake.associateResolverRuleWithContextArgsForCall)]
	fake.associateResolverRuleWithContextArgsForCall = append(fake.associateResolverRuleWithContextArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.AssociateResolverRuleWithContextStub
	fakeReturns := fake.associateResolverRuleWithContextReturns
	fake.recordInvocation("AssociateResolverRuleWithContext", []interface{}{arg1, arg2, arg3, arg4})
	fake.associateResolverRuleWithContextMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResolverClient) AssociateResolverRuleWithContextCallCount() int {
	fake.associateResolverRuleWithContextMutex.RLock()
	defer fake.associateResolverRuleWithContextMutex.RUnlock()
	return len(fake.associateResolverRuleWithContextArgsForCall)
}

func (fake *FakeResolverClient) AssociateResolverRuleWithContextCalls(stub func(context.Context, string, string, string) error) {
	fake.associateResolverRuleWithContextMutex.Lock()
	defer fake.associateResolverRuleWithContextMutex.Unlock()
	fake.AssociateResolverRuleWithContextStub = stub
}

func (fake *FakeResolverClient) AssociateResolverRuleWithContextArgsForCall(i int) (context.Context, string, string, string) {
	fake.associateResolverRuleWithContextMutex.RLock()
	defer fake.associateResolverRuleWithContextMutex.RUnlock()
	argsForCall := fake.associateResolverRuleWithContextArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeResolverClient) AssociateResolverRuleWithContextReturns(result1 error) {
	fake.associateResolverRuleWithContextMutex.Lock()
	defer fake.associateResolverRuleWithContextMutex.Unlock()
	fake.AssociateResolverRuleWithContextStub = nil
	fake.associateResolverRuleWithContextReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResolverClient) AssociateResolverRuleWithContextReturnsOnCall(i int, result1 error) {
	fake.associateResolverRuleWithContextMutex.Lock()
	defer fake.associateResolverRuleWithContextMutex.Unlock()
	fake.AssociateResolverRuleWithContextStub = nil
	if fake.associateResolverRuleWithContextReturnsOnCall == nil {
		fake.associateResolverRuleWithContextReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.associateResolverRuleWithContextReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResolverClient) CreateResolverRule(arg1 context.Context, arg2 logr.Logger, arg3 resolver.Cluster, arg4 string, arg5 string, arg6 string) (string, string, error) {
	fake.createResolverRuleMutex.Lock()
	ret, specificReturn := fake.createResolverRuleReturnsOnCall[len(fake.createResolverRuleArgsForCall)]
	fake.createResolverRuleArgsForCall = append(fake.createResolverRuleArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 resolver.Cluster
		arg4 string
		arg5 string
		arg6 string
	}{arg1, arg2, arg3, arg4, arg5, arg6})
	stub := fake.CreateResolverRuleStub
	fakeReturns := fake.createResolverRuleReturns
	fake.recordInvocation("CreateResolverRule", []interface{}{arg1, arg2, arg3, arg4, arg5, arg6})
	fake.createResolverRuleMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4, arg5, arg6)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *FakeResolverClient) CreateResolverRuleCallCount() int {
	fake.createResolverRuleMutex.RLock()
	defer fake.createResolverRuleMutex.RUnlock()
	return len(fake.createResolverRuleArgsForCall)
}

func (fake *FakeResolverClient) CreateResolverRuleCalls(stub func(context.Context, logr.Logger, resolver.Cluster, string, string, string) (string, string, error)) {
	fake.createResolverRuleMutex.Lock()
	defer fake.createResolverRuleMutex.Unlock()
	fake.CreateResolverRuleStub = stub
}

func (fake *FakeResolverClient) CreateResolverRuleArgsForCall(i int) (context.Context, logr.Logger, resolver.Cluster, string, string, string) {
	fake.createResolverRuleMutex.RLock()
	defer fake.createResolverRuleMutex.RUnlock()
	argsForCall := fake.createResolverRuleArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4, argsForCall.arg5, argsForCall.arg6
}

func (fake *FakeResolverClient) CreateResolverRuleReturns(result1 string, result2 string, result3 error) {
	fake.createResolverRuleMutex.Lock()
	defer fake.createResolverRuleMutex.Unlock()
	fake.CreateResolverRuleStub = nil
	fake.createResolverRuleReturns = struct {
		result1 string
		result2 string
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeResolverClient) CreateResolverRuleReturnsOnCall(i int, result1 string, result2 string, result3 error) {
	fake.createResolverRuleMutex.Lock()
	defer fake.createResolverRuleMutex.Unlock()
	fake.CreateResolverRuleStub = nil
	if fake.createResolverRuleReturnsOnCall == nil {
		fake.createResolverRuleReturnsOnCall = make(map[int]struct {
			result1 string
			result2 string
			result3 error
		})
	}
	fake.createResolverRuleReturnsOnCall[i] = struct {
		result1 string
		result2 string
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeResolverClient) DeleteResolverRule(arg1 context.Context, arg2 logr.Logger, arg3 resolver.Cluster, arg4 string) error {
	fake.deleteResolverRuleMutex.Lock()
	ret, specificReturn := fake.deleteResolverRuleReturnsOnCall[len(fake.deleteResolverRuleArgsForCall)]
	fake.deleteResolverRuleArgsForCall = append(fake.deleteResolverRuleArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 resolver.Cluster
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.DeleteResolverRuleStub
	fakeReturns := fake.deleteResolverRuleReturns
	fake.recordInvocation("DeleteResolverRule", []interface{}{arg1, arg2, arg3, arg4})
	fake.deleteResolverRuleMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResolverClient) DeleteResolverRuleCallCount() int {
	fake.deleteResolverRuleMutex.RLock()
	defer fake.deleteResolverRuleMutex.RUnlock()
	return len(fake.deleteResolverRuleArgsForCall)
}

func (fake *FakeResolverClient) DeleteResolverRuleCalls(stub func(context.Context, logr.Logger, resolver.Cluster, string) error) {
	fake.deleteResolverRuleMutex.Lock()
	defer fake.deleteResolverRuleMutex.Unlock()
	fake.DeleteResolverRuleStub = stub
}

func (fake *FakeResolverClient) DeleteResolverRuleArgsForCall(i int) (context.Context, logr.Logger, resolver.Cluster, string) {
	fake.deleteResolverRuleMutex.RLock()
	defer fake.deleteResolverRuleMutex.RUnlock()
	argsForCall := fake.deleteResolverRuleArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeResolverClient) DeleteResolverRuleReturns(result1 error) {
	fake.deleteResolverRuleMutex.Lock()
	defer fake.deleteResolverRuleMutex.Unlock()
	fake.DeleteResolverRuleStub = nil
	fake.deleteResolverRuleReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResolverClient) DeleteResolverRuleReturnsOnCall(i int, result1 error) {
	fake.deleteResolverRuleMutex.Lock()
	defer fake.deleteResolverRuleMutex.Unlock()
	fake.DeleteResolverRuleStub = nil
	if fake.deleteResolverRuleReturnsOnCall == nil {
		fake.deleteResolverRuleReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteResolverRuleReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResolverClient) DisassociateResolverRuleWithContext(arg1 context.Context, arg2 string, arg3 string) error {
	fake.disassociateResolverRuleWithContextMutex.Lock()
	ret, specificReturn := fake.disassociateResolverRuleWithContextReturnsOnCall[len(fake.disassociateResolverRuleWithContextArgsForCall)]
	fake.disassociateResolverRuleWithContextArgsForCall = append(fake.disassociateResolverRuleWithContextArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.DisassociateResolverRuleWithContextStub
	fakeReturns := fake.disassociateResolverRuleWithContextReturns
	fake.recordInvocation("DisassociateResolverRuleWithContext", []interface{}{arg1, arg2, arg3})
	fake.disassociateResolverRuleWithContextMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResolverClient) DisassociateResolverRuleWithContextCallCount() int {
	fake.disassociateResolverRuleWithContextMutex.RLock()
	defer fake.disassociateResolverRuleWithContextMutex.RUnlock()
	return len(fake.disassociateResolverRuleWithContextArgsForCall)
}

func (fake *FakeResolverClient) DisassociateResolverRuleWithContextCalls(stub func(context.Context, string, string) error) {
	fake.disassociateResolverRuleWithContextMutex.Lock()
	defer fake.disassociateResolverRuleWithContextMutex.Unlock()
	fake.DisassociateResolverRuleWithContextStub = stub
}

func (fake *FakeResolverClient) DisassociateResolverRuleWithContextArgsForCall(i int) (context.Context, string, string) {
	fake.disassociateResolverRuleWithContextMutex.RLock()
	defer fake.disassociateResolverRuleWithContextMutex.RUnlock()
	argsForCall := fake.disassociateResolverRuleWithContextArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeResolverClient) DisassociateResolverRuleWithContextReturns(result1 error) {
	fake.disassociateResolverRuleWithContextMutex.Lock()
	defer fake.disassociateResolverRuleWithContextMutex.Unlock()
	fake.DisassociateResolverRuleWithContextStub = nil
	fake.disassociateResolverRuleWithContextReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResolverClient) DisassociateResolverRuleWithContextReturnsOnCall(i int, result1 error) {
	fake.disassociateResolverRuleWithContextMutex.Lock()
	defer fake.disassociateResolverRuleWithContextMutex.Unlock()
	fake.DisassociateResolverRuleWithContextStub = nil
	if fake.disassociateResolverRuleWithContextReturnsOnCall == nil {
		fake.disassociateResolverRuleWithContextReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.disassociateResolverRuleWithContextReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResolverClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.associateResolverRuleWithContextMutex.RLock()
	defer fake.associateResolverRuleWithContextMutex.RUnlock()
	fake.createResolverRuleMutex.RLock()
	defer fake.createResolverRuleMutex.RUnlock()
	fake.deleteResolverRuleMutex.RLock()
	defer fake.deleteResolverRuleMutex.RUnlock()
	fake.disassociateResolverRuleWithContextMutex.RLock()
	defer fake.disassociateResolverRuleWithContextMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeResolverClient) recordInvocation(key string, args []interface{}) {
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

var _ resolver.ResolverClient = new(FakeResolverClient)
