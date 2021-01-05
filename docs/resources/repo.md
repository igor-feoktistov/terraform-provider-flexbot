---
page_title: "repo Resource"
---

# Repo Resource

Provides Flexbot Repo resource to manage images and templates repositories. This can be used to upload, update, and remove images and templates on Flexbot storage.
If you previously used `flexbot` CLI tool to manage images and templates, you can shift this management entirely to Terraform.

## Example Usage

### Create empty repository

You can check existent repositories in Terraform State. Look for `templates` and `images` computed attributes.

```hcl
resource "flexbot_repo" "repo" {}
```

### Take over repository management via Terraform

Empty `location` attributes prevent accidental updates to images and templates.

```hcl
resource "flexbot_repo" "repo" {
  image_repo {
    name = "rhel-7.8.01-iboot"
  }
  image_repo {
    name = "ubuntu-18.04.05.01-iboot"
  }
  template_repo {
    name = "rhel7.8.01-cloud-init.template"
  }
  template_repo {
    name = "ubuntu-18.04.05.01-cloud-init.template"
  }
}
```

### Upload new image and new template into repository

Please be aware that changed non-empty `location` attribute will trigger respective image or template update in repository.

**Note** You may want to upload images from a client on local network with cDOT storage, or at least on network with decent bandwidth.

```hcl
resource "flexbot_repo" "repo" {
  image_repo {
    name = "rhel-7.8.01-iboot"
  }
  image_repo {
    name = "ubuntu-18.04.05.01-iboot"
  }
  image_repo {
    name = "ubuntu-18.04.05.02-iboot"
    location = "/diskimage-builder/images/ubuntu-18.04.05.02-iboot.raw"
  }
  template_repo {
    name = "rhel7.8.01-cloud-init.template"
  }
  template_repo {
    name = "ubuntu-18.04.05.01-cloud-init.template"
  }
  template_repo {
    name = "ubuntu-18.04.05.02-cloud-init.template"
    location = "/diskimage-builder/templates/ubuntu-18.04.05.02-cloud-init.template"
  }
}
```

### Remove old image and old template

```hcl
resource "flexbot_repo" "repo" {
  image_repo {
    name = "rhel-7.8.01-iboot"
  }
  image_repo {
    name = "ubuntu-18.04.05.02-iboot"
  }
  template_repo {
    name = "rhel7.8.01-cloud-init.template"
  }
  template_repo {
    name = "ubuntu-18.04.05.02-cloud-init.template"
  }
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) Name of the image or template. You will reference these names in `server` resource.
* `location` - (Optional) Need only to upload or update image or template. It is recommended to change the value to empty after that.
