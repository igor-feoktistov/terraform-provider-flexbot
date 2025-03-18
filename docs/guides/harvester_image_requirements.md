---
page_title: "Harvester Image Requirements"
---

# Harvester Image Requirements

Harvester image in this project is NetApp storage LUN with a content copied from modified Harvester Live ISO image.

### Steps to modify Harvester Live ISO:

* Modify `/boot/initrd`
  * Unpack `/boot/initrd`
    Run command `mkdir initrd-iboot; cd initrd-iboot; lsinitrd --unpack ../initrd`
  * Patch `/sbin/dmsquash-live-root`
* Modify `/boot/grub2/grub.cfg`   
  * Pack `/boot/initrd`
    Run command `cd initrd-iboot; find . 2>/dev/null | cpio -o -c -R root:root > ../initrd.iboot`
  
* Modify `/boot/grub2/grub.cfg`
  Add kernel arguments `rd.iscsi.firmware rd.iscsi.ibft`
  
* Modify `rootfs.squashfs`
  * Unpack `rootfs.squashfs`
    Run command `unsquashfs -d rootfs.squashfs-iboot rootfs.squashfs`
  * Compile adapted to `cloud-init` `nocloud` [harvester-installer](https://github.com/igor-feoktistov/harvester-installer-v1.4.1)
  * Copy compiled harvester-installer to `/usr/local/bin` in `rootfs.squashfs`
  * Pack `rootfs.squashfs`
    Run command `mksquashfs rootfs.squashfs-iboot rootfs.squashfs.iboot`

* Re-pack Live ISO image
  * I recommend using `xorriso`:
    Create `overlay` directory and copy there modified above files:
    ```
    overlay
    ├── boot
    │   ├── grub2
    │   │   └── grub.cfg
    │   └── initrd
    └── rootfs.squashfs
    ```
    Run command `xorriso -indev harvester-v1.4.1-amd64.iso -outdev harvester-v1.4.1-amd64-iboot.iso -map overlay / -boot_image any replay`
  