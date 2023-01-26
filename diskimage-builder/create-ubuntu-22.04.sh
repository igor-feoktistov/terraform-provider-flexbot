export IMAGE=22.04
export DIB_RELEASE=jammy
export DIB_DEV_USER_USERNAME=devuser
export DIB_DEV_USER_PWDLESS_SUDO=Yes
export DIB_DEV_USER_PASSWORD=secret
export ELEMENTS_PATH=/usr/local/diskimage-builder/elements/ubuntu-${IMAGE}
export DIB_BLOCK_DEVICE_CONFIG='
  - local_loop:
      name: image0
      size: 3.5GiB
      mkfs:
        name: rootfs
        label: rootfs
        type: ext4
        mount:
          mount_point: /
          fstab:
            options: "defaults"
            fsck-passno: 1'
disk-image-create -x -a amd64 vm block-device-mbr ubuntu \
  cloud-init-nocloud devuser iscsi-boot \
  bootloader grub2 install-static runtime-ssh-host-keys \
  -p multipath-tools \
  -p multipath-tools-boot \
  -p kpartx-boot \
  -p net-tools \
  -p sysstat \
  -p cloud-utils \
  -p cloud-initramfs-growroot \
  -p nfs-common \
  -p chrony \
  -t raw -o images/ubuntu-${IMAGE}-iboot.raw
