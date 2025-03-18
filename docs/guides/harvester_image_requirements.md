---
page_title: "Harvester Image Requirements"
---

# Harvester Image Requirements

Harvester image in this project is NetApp iSCSI LUN with a content copied from modified Harvester Live ISO image.

### Steps to modify Harvester Live ISO:

* Modify `/boot/initrd`
  * Unpack `/boot/initrd`  
    Run command `mkdir initrd-iboot; cd initrd-iboot; lsinitrd --unpack ../initrd`
  * Patch `/sbin/dmsquash-live-root` (see patch below)
  * Pack `/boot/initrd`  
    Run command `cd initrd-iboot; find . 2>/dev/null | cpio -o -c -R root:root > ../initrd.iboot`

* Modify `/boot/grub2/grub.cfg`  
  Add kernel arguments `rd.iscsi.firmware rd.iscsi.ibft`
  
* Modify `rootfs.squashfs`
  * Unpack `rootfs.squashfs`  
    Run command `unsquashfs -d rootfs.squashfs-iboot rootfs.squashfs`
  * Compile adapted to `cloud-init` `nocloud` [harvester-installer](https://github.com/igor-feoktistov/harvester-installer-v1.4.1)
  * Copy compiled [harvester-installer](https://github.com/igor-feoktistov/harvester-installer-v1.4.1) to `/usr/local/bin` in `rootfs.squashfs`
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

#### Patch for `/sbin/dmsquash-live-root`
```{r, echo=TRUE}
--- dmsquash-live-root.ORIG	2025-02-07 00:47:03.648808073 +0000
+++ dmsquash-live-root	2025-02-10 16:56:26.861697977 +0000
@@ -89,8 +89,7 @@
         SQUASHED=$livedev
     elif [ "$livedev_fstype" != "ntfs" ]; then
         if ! mount -n -t "$fstype" -o "${liverw:-ro}" "$livedev" /run/initramfs/live; then
-            die "Failed to mount block device of live image"
-            exit 1
+            mount -o ro `blkid --label COS_LIVE`-part1 /run/initramfs/live
         fi
     else
         # Symlinking /usr/bin/ntfs-3g as /sbin/mount.ntfs seems to boot

```
