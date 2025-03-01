// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go

// Package mock_provider is a generated GoMock package.
package mock_provider

import (
	context "context"
	reflect "reflect"

	provider "github.com/filecoin-project/index-provider"
	metadata "github.com/filecoin-project/index-provider/metadata"
	schema "github.com/filecoin-project/storetheindex/api/v0/ingest/schema"
	gomock "github.com/golang/mock/gomock"
	cid "github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p-core/peer"
	multihash "github.com/multiformats/go-multihash"
)

// MockInterface is a mock of Interface interface.
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface.
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance.
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// GetAdv mocks base method.
func (m *MockInterface) GetAdv(arg0 context.Context, arg1 cid.Cid) (*schema.Advertisement, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAdv", arg0, arg1)
	ret0, _ := ret[0].(*schema.Advertisement)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAdv indicates an expected call of GetAdv.
func (mr *MockInterfaceMockRecorder) GetAdv(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAdv", reflect.TypeOf((*MockInterface)(nil).GetAdv), arg0, arg1)
}

// GetLatestAdv mocks base method.
func (m *MockInterface) GetLatestAdv(arg0 context.Context) (cid.Cid, *schema.Advertisement, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestAdv", arg0)
	ret0, _ := ret[0].(cid.Cid)
	ret1, _ := ret[1].(*schema.Advertisement)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetLatestAdv indicates an expected call of GetLatestAdv.
func (mr *MockInterfaceMockRecorder) GetLatestAdv(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestAdv", reflect.TypeOf((*MockInterface)(nil).GetLatestAdv), arg0)
}

// NotifyPut mocks base method.
func (m *MockInterface) NotifyPut(ctx context.Context, provider *peer.AddrInfo, contextID []byte, md metadata.Metadata) (cid.Cid, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NotifyPut", ctx, provider, contextID, md)
	ret0, _ := ret[0].(cid.Cid)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NotifyPut indicates an expected call of NotifyPut.
func (mr *MockInterfaceMockRecorder) NotifyPut(ctx, provider, contextID, md interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotifyPut", reflect.TypeOf((*MockInterface)(nil).NotifyPut), ctx, provider, contextID, md)
}

// NotifyRemove mocks base method.
func (m *MockInterface) NotifyRemove(ctx context.Context, providerID peer.ID, contextID []byte) (cid.Cid, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NotifyRemove", ctx, providerID, contextID)
	ret0, _ := ret[0].(cid.Cid)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NotifyRemove indicates an expected call of NotifyRemove.
func (mr *MockInterfaceMockRecorder) NotifyRemove(ctx, providerID, contextID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotifyRemove", reflect.TypeOf((*MockInterface)(nil).NotifyRemove), ctx, providerID, contextID)
}

// Publish mocks base method.
func (m *MockInterface) Publish(arg0 context.Context, arg1 schema.Advertisement) (cid.Cid, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", arg0, arg1)
	ret0, _ := ret[0].(cid.Cid)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Publish indicates an expected call of Publish.
func (mr *MockInterfaceMockRecorder) Publish(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockInterface)(nil).Publish), arg0, arg1)
}

// PublishLocal mocks base method.
func (m *MockInterface) PublishLocal(arg0 context.Context, arg1 schema.Advertisement) (cid.Cid, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishLocal", arg0, arg1)
	ret0, _ := ret[0].(cid.Cid)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PublishLocal indicates an expected call of PublishLocal.
func (mr *MockInterfaceMockRecorder) PublishLocal(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishLocal", reflect.TypeOf((*MockInterface)(nil).PublishLocal), arg0, arg1)
}

// RegisterMultihashLister mocks base method.
func (m *MockInterface) RegisterMultihashLister(arg0 provider.MultihashLister) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterMultihashLister", arg0)
}

// RegisterMultihashLister indicates an expected call of RegisterMultihashLister.
func (mr *MockInterfaceMockRecorder) RegisterMultihashLister(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterMultihashLister", reflect.TypeOf((*MockInterface)(nil).RegisterMultihashLister), arg0)
}

// Shutdown mocks base method.
func (m *MockInterface) Shutdown() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Shutdown")
	ret0, _ := ret[0].(error)
	return ret0
}

// Shutdown indicates an expected call of Shutdown.
func (mr *MockInterfaceMockRecorder) Shutdown() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Shutdown", reflect.TypeOf((*MockInterface)(nil).Shutdown))
}

// MockMultihashIterator is a mock of MultihashIterator interface.
type MockMultihashIterator struct {
	ctrl     *gomock.Controller
	recorder *MockMultihashIteratorMockRecorder
}

// MockMultihashIteratorMockRecorder is the mock recorder for MockMultihashIterator.
type MockMultihashIteratorMockRecorder struct {
	mock *MockMultihashIterator
}

// NewMockMultihashIterator creates a new mock instance.
func NewMockMultihashIterator(ctrl *gomock.Controller) *MockMultihashIterator {
	mock := &MockMultihashIterator{ctrl: ctrl}
	mock.recorder = &MockMultihashIteratorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMultihashIterator) EXPECT() *MockMultihashIteratorMockRecorder {
	return m.recorder
}

// Next mocks base method.
func (m *MockMultihashIterator) Next() (multihash.Multihash, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].(multihash.Multihash)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Next indicates an expected call of Next.
func (mr *MockMultihashIteratorMockRecorder) Next() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockMultihashIterator)(nil).Next))
}
