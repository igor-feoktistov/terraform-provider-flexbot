# OS images building procedure

### Prerequisites
* Python 3.6 or higher
* Installed [diskimage-builder](https://docs.openstack.org/diskimage-builder/latest/) v3.11 or higher

### Steps to build OS image:
* Clone content of this folder to `/usr/local/diskimage-builder` directory.
* Run script [create-ubuntu-20.04-iboot.sh](create-ubuntu-20.04-iboot.sh).
* Newly created RAW image is ready to be submitted to image repository.
