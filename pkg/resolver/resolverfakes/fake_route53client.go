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
	AddDnsRecordsToHostedZoneStub        func(context.Context, logr.Logger, string, []resolver.DNSRecord) error
	addDnsRecordsToHostedZoneMutex       sync.RWMutex
	addDnsRecordsToHostedZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 []resolver.DNSRecord
	}
	addDnsRecordsToHostedZoneReturns struct {
		result1 error
	}
	addDnsRecordsToHostedZoneReturnsOnCall map[int]struct {
		result1 error
	}
	CreateHostedZoneStub        func(context.Context, logr.Logger, resolver.DnsZone) (string, error)
	createHostedZoneMutex       sync.RWMutex
	createHostedZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 resolver.DnsZone
	}
	createHostedZoneReturns struct {
		result1 string
		result2 error
	}
	createHostedZoneReturnsOnCall map[int]struct {
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
	DeleteDnsRecordsFromHostedZoneStub        func(context.Context, logr.Logger, string, []resolver.DNSRecord) error
	deleteDnsRecordsFromHostedZoneMutex       sync.RWMutex
	deleteDnsRecordsFromHostedZoneArgsForCall []struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 []resolver.DNSRecord
	}
	deleteDnsRecordsFromHostedZoneReturns struct {
		result1 error
	}
	deleteDnsRecordsFromHostedZoneReturnsOnCall map[int]struct {
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

func (fake *FakeRoute53Client) AddDnsRecordsToHostedZone(arg1 context.Context, arg2 logr.Logger, arg3 string, arg4 []resolver.DNSRecord) error {
	var arg4Copy []resolver.DNSRecord
	if arg4 != nil {
		arg4Copy = make([]resolver.DNSRecord, len(arg4))
		copy(arg4Copy, arg4)
	}
	fake.addDnsRecordsToHostedZoneMutex.Lock()
	ret, specificReturn := fake.addDnsRecordsToHostedZoneReturnsOnCall[len(fake.addDnsRecordsToHostedZoneArgsForCall)]
	fake.addDnsRecordsToHostedZoneArgsForCall = append(fake.addDnsRecordsToHostedZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 []resolver.DNSRecord
	}{arg1, arg2, arg3, arg4Copy})
	stub := fake.AddDnsRecordsToHostedZoneStub
	fakeReturns := fake.addDnsRecordsToHostedZoneReturns
	fake.recordInvocation("AddDnsRecordsToHostedZone", []interface{}{arg1, arg2, arg3, arg4Copy})
	fake.addDnsRecordsToHostedZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeRoute53Client) AddDnsRecordsToHostedZoneCallCount() int {
	fake.addDnsRecordsToHostedZoneMutex.RLock()
	defer fake.addDnsRecordsToHostedZoneMutex.RUnlock()
	return len(fake.addDnsRecordsToHostedZoneArgsForCall)
}

func (fake *FakeRoute53Client) AddDnsRecordsToHostedZoneCalls(stub func(context.Context, logr.Logger, string, []resolver.DNSRecord) error) {
	fake.addDnsRecordsToHostedZoneMutex.Lock()
	defer fake.addDnsRecordsToHostedZoneMutex.Unlock()
	fake.AddDnsRecordsToHostedZoneStub = stub
}

func (fake *FakeRoute53Client) AddDnsRecordsToHostedZoneArgsForCall(i int) (context.Context, logr.Logger, string, []resolver.DNSRecord) {
	fake.addDnsRecordsToHostedZoneMutex.RLock()
	defer fake.addDnsRecordsToHostedZoneMutex.RUnlock()
	argsForCall := fake.addDnsRecordsToHostedZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeRoute53Client) AddDnsRecordsToHostedZoneReturns(result1 error) {
	fake.addDnsRecordsToHostedZoneMutex.Lock()
	defer fake.addDnsRecordsToHostedZoneMutex.Unlock()
	fake.AddDnsRecordsToHostedZoneStub = nil
	fake.addDnsRecordsToHostedZoneReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) AddDnsRecordsToHostedZoneReturnsOnCall(i int, result1 error) {
	fake.addDnsRecordsToHostedZoneMutex.Lock()
	defer fake.addDnsRecordsToHostedZoneMutex.Unlock()
	fake.AddDnsRecordsToHostedZoneStub = nil
	if fake.addDnsRecordsToHostedZoneReturnsOnCall == nil {
		fake.addDnsRecordsToHostedZoneReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.addDnsRecordsToHostedZoneReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) CreateHostedZone(arg1 context.Context, arg2 logr.Logger, arg3 resolver.DnsZone) (string, error) {
	fake.createHostedZoneMutex.Lock()
	ret, specificReturn := fake.createHostedZoneReturnsOnCall[len(fake.createHostedZoneArgsForCall)]
	fake.createHostedZoneArgsForCall = append(fake.createHostedZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 resolver.DnsZone
	}{arg1, arg2, arg3})
	stub := fake.CreateHostedZoneStub
	fakeReturns := fake.createHostedZoneReturns
	fake.recordInvocation("CreateHostedZone", []interface{}{arg1, arg2, arg3})
	fake.createHostedZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeRoute53Client) CreateHostedZoneCallCount() int {
	fake.createHostedZoneMutex.RLock()
	defer fake.createHostedZoneMutex.RUnlock()
	return len(fake.createHostedZoneArgsForCall)
}

func (fake *FakeRoute53Client) CreateHostedZoneCalls(stub func(context.Context, logr.Logger, resolver.DnsZone) (string, error)) {
	fake.createHostedZoneMutex.Lock()
	defer fake.createHostedZoneMutex.Unlock()
	fake.CreateHostedZoneStub = stub
}

func (fake *FakeRoute53Client) CreateHostedZoneArgsForCall(i int) (context.Context, logr.Logger, resolver.DnsZone) {
	fake.createHostedZoneMutex.RLock()
	defer fake.createHostedZoneMutex.RUnlock()
	argsForCall := fake.createHostedZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeRoute53Client) CreateHostedZoneReturns(result1 string, result2 error) {
	fake.createHostedZoneMutex.Lock()
	defer fake.createHostedZoneMutex.Unlock()
	fake.CreateHostedZoneStub = nil
	fake.createHostedZoneReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeRoute53Client) CreateHostedZoneReturnsOnCall(i int, result1 string, result2 error) {
	fake.createHostedZoneMutex.Lock()
	defer fake.createHostedZoneMutex.Unlock()
	fake.CreateHostedZoneStub = nil
	if fake.createHostedZoneReturnsOnCall == nil {
		fake.createHostedZoneReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.createHostedZoneReturnsOnCall[i] = struct {
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

func (fake *FakeRoute53Client) DeleteDnsRecordsFromHostedZone(arg1 context.Context, arg2 logr.Logger, arg3 string, arg4 []resolver.DNSRecord) error {
	var arg4Copy []resolver.DNSRecord
	if arg4 != nil {
		arg4Copy = make([]resolver.DNSRecord, len(arg4))
		copy(arg4Copy, arg4)
	}
	fake.deleteDnsRecordsFromHostedZoneMutex.Lock()
	ret, specificReturn := fake.deleteDnsRecordsFromHostedZoneReturnsOnCall[len(fake.deleteDnsRecordsFromHostedZoneArgsForCall)]
	fake.deleteDnsRecordsFromHostedZoneArgsForCall = append(fake.deleteDnsRecordsFromHostedZoneArgsForCall, struct {
		arg1 context.Context
		arg2 logr.Logger
		arg3 string
		arg4 []resolver.DNSRecord
	}{arg1, arg2, arg3, arg4Copy})
	stub := fake.DeleteDnsRecordsFromHostedZoneStub
	fakeReturns := fake.deleteDnsRecordsFromHostedZoneReturns
	fake.recordInvocation("DeleteDnsRecordsFromHostedZone", []interface{}{arg1, arg2, arg3, arg4Copy})
	fake.deleteDnsRecordsFromHostedZoneMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeRoute53Client) DeleteDnsRecordsFromHostedZoneCallCount() int {
	fake.deleteDnsRecordsFromHostedZoneMutex.RLock()
	defer fake.deleteDnsRecordsFromHostedZoneMutex.RUnlock()
	return len(fake.deleteDnsRecordsFromHostedZoneArgsForCall)
}

func (fake *FakeRoute53Client) DeleteDnsRecordsFromHostedZoneCalls(stub func(context.Context, logr.Logger, string, []resolver.DNSRecord) error) {
	fake.deleteDnsRecordsFromHostedZoneMutex.Lock()
	defer fake.deleteDnsRecordsFromHostedZoneMutex.Unlock()
	fake.DeleteDnsRecordsFromHostedZoneStub = stub
}

func (fake *FakeRoute53Client) DeleteDnsRecordsFromHostedZoneArgsForCall(i int) (context.Context, logr.Logger, string, []resolver.DNSRecord) {
	fake.deleteDnsRecordsFromHostedZoneMutex.RLock()
	defer fake.deleteDnsRecordsFromHostedZoneMutex.RUnlock()
	argsForCall := fake.deleteDnsRecordsFromHostedZoneArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeRoute53Client) DeleteDnsRecordsFromHostedZoneReturns(result1 error) {
	fake.deleteDnsRecordsFromHostedZoneMutex.Lock()
	defer fake.deleteDnsRecordsFromHostedZoneMutex.Unlock()
	fake.DeleteDnsRecordsFromHostedZoneStub = nil
	fake.deleteDnsRecordsFromHostedZoneReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeRoute53Client) DeleteDnsRecordsFromHostedZoneReturnsOnCall(i int, result1 error) {
	fake.deleteDnsRecordsFromHostedZoneMutex.Lock()
	defer fake.deleteDnsRecordsFromHostedZoneMutex.Unlock()
	fake.DeleteDnsRecordsFromHostedZoneStub = nil
	if fake.deleteDnsRecordsFromHostedZoneReturnsOnCall == nil {
		fake.deleteDnsRecordsFromHostedZoneReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteDnsRecordsFromHostedZoneReturnsOnCall[i] = struct {
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
	fake.addDnsRecordsToHostedZoneMutex.RLock()
	defer fake.addDnsRecordsToHostedZoneMutex.RUnlock()
	fake.createHostedZoneMutex.RLock()
	defer fake.createHostedZoneMutex.RUnlock()
	fake.deleteDelegationFromParentZoneMutex.RLock()
	defer fake.deleteDelegationFromParentZoneMutex.RUnlock()
	fake.deleteDnsRecordsFromHostedZoneMutex.RLock()
	defer fake.deleteDnsRecordsFromHostedZoneMutex.RUnlock()
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
