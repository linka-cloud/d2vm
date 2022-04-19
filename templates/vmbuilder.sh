#!/usr/bin/env bash

OUT="/dev/null"

if [[ -n "$DEBUG" ]]; then
  set -x
  OUT="/dev/stderr"
fi
set -e

SRC=${1:-rootfs.tar}
DISK_NAME=${2:-disk0}
SIZE=${3:-10G}

BLOCK="$DISK_NAME.raw"
QCOW2="$DISK_NAME.qcow2"
MOUNTPOINT=/mnt

cleanup() {
  rm -rf $BLOCK
}

make_img() {
  echo "Creating raw image of size $SIZE"
  fallocate -l $SIZE $BLOCK &> $OUT
  (
  echo n # Add a new partition
  echo p # Primary partition
  echo 1 # Partition number
  echo   # First sector (Accept default: 1)
  echo   # Last sector (Accept default: varies)
  echo a
  echo w # Write changes
  ) | fdisk $BLOCK &> $OUT
}

mount_img() {
  echo "Mounting image"
  DEVICE_ROOT=$(losetup --show -f $BLOCK)
  kpartx -v -a $DEVICE_ROOT &> $OUT
  DEVICE=/dev/mapper/"$(basename ${DEVICE_ROOT})p1"
  mkfs.ext4 $DEVICE &> $OUT
  mount $DEVICE $MOUNTPOINT
}

unmount_img() {
  echo "Unmounting image"
  umount $MOUNTPOINT/
  kpartx -d $DEVICE_ROOT &> $OUT
  losetup -d $DEVICE_ROOT &> $OUT
}

copy_rootfs() {
  echo "Copying root file system"
  tar -xvf $SRC -C $MOUNTPOINT &> $OUT
}

setup_rootfs() {
  echo "Setting up root file system"
  uuid=$(blkid -s UUID -o value $DEVICE)

  mkdir -p $MOUNTPOINT/etc/
  echo "UUID=$uuid / ext4 errors=remount-ro 0 1" > $MOUNTPOINT/etc/fstab

  echo "nameserver 8.8.8.8" > $MOUNTPOINT/etc/resolv.conf

  echo localhost > $MOUNTPOINT/etc/hostname

  cat <<EOF > $MOUNTPOINT/etc/hosts
127.0.0.1 localhost

# The following lines are desirable for IPv6 capable hosts
::1 ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
EOF

  rm -rf $MOUNTPOINT/usr/sbin/policy-rc.d
}

install_kernel() {
  echo "Installing linux kernel"
  extlinux --install $MOUNTPOINT/boot/ &> $OUT

  cat <<EOF > $MOUNTPOINT/boot/syslinux.cfg
DEFAULT linux
  SAY Now booting the kernel from EXTLINUX...
 LABEL linux
  KERNEL /boot/vmlinuz
  APPEND ro root=/dev/sda1 initrd=/boot/initrd.img net.ifnames=0 console=tty0 console=ttyS0,115200n8
EOF
}

setup_mbr() {
  echo "Setting up boot loader"
  dd if=/usr/lib/EXTLINUX/mbr.bin of=$BLOCK bs=440 count=1 conv=notrunc &> $OUT
}

convert() {
  echo "Converting image to QCOW2"
  echo ""
  qemu-img convert $BLOCK -O qcow2 $QCOW2
}

cleanup
make_img
mount_img
copy_rootfs
setup_rootfs
install_kernel
unmount_img
setup_mbr
convert
