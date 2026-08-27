//go:build !linux

// Package smbmount 非 Linux 平台 stub (开发机通常是 Windows/macOS, 不支持
// mount.cifs). 提供编译期满足 + 运行时直接返回 ErrMountBinMissing.
// 共享类型 Mounter/MountRequest/MountStatus/DefaultMountTimeout/
// ErrMountBinMissing 定义在 service.go（无 build tag）。
package smbmount

import (
	"context"
)

// NewExecMounter stub 实现: 所有调用都返回平台不支持错误.
func NewExecMounter() Mounter { return &stubMounter{} }

type stubMounter struct{}

func (s *stubMounter) Mount(context.Context, MountRequest) error { return ErrMountBinMissing }
func (s *stubMounter) Unmount(context.Context, string) error      { return ErrMountBinMissing }
func (s *stubMounter) IsMounted(string) (bool, error)             { return false, ErrMountBinMissing }
func (s *stubMounter) Refresh(context.Context, string) (MountStatus, error) {
	return MountStatus{}, ErrMountBinMissing
}
