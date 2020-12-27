---
layout: default
title: Fedora CoreOS v1.0.0
parent: Configuration specifications
nav_order: 49
---

# Fedora CoreOS Specification v1.0.0

The Fedora CoreOS configuration is a YAML document conforming to the following specification, with **_italicized_** entries being optional:

* **variant** (string): used to differentiate configs for different operating systems. Must be `fcos` for this specification.
* **version** (string): the semantic version of the spec for this document. This document is for version `1.0.0` and generates Ignition configs with version `3.0.0`.
* **ignition** (object): metadata about the configuration itself.
  * **_config_** (objects): options related to the configuration.
    * **_merge_** (list of objects): a list of the configs to be merged to the current config.
      * **source** (string): the URL of the config. Supported schemes are `http`, `https`, `s3`, `tftp`, and [`data`][rfc2397]. Note: When using `http`, it is advisable to use the verification option to ensure the contents haven't been modified.
      * **_verification_** (object): options related to the verification of the config.
        * **_hash_** (string): the hash of the config, in the form `<type>-<value>` where type is `sha512`.
    * **_replace_** (object): the config that will replace the current.
      * **source** (string): the URL of the config. Supported schemes are `http`, `https`, `s3`, `tftp`, and [`data`][rfc2397]. Note: When using `http`, it is advisable to use the verification option to ensure the contents haven't been modified.
      * **_verification_** (object): options related to the verification of the config.
        * **_hash_** (string): the hash of the config, in the form `<type>-<value>` where type is `sha512`.
  * **_timeouts_** (object): options relating to `http` timeouts when fetching files over `http` or `https`.
    * **_http_response_headers_** (integer) the time to wait (in seconds) for the server's response headers (but not the body) after making a request. 0 indicates no timeout. Default is 10 seconds.
    * **_http_total_** (integer) the time limit (in seconds) for the operation (connection, request, and response), including retries. 0 indicates no timeout. Default is 0.
  * **_security_** (object): options relating to network security.
    * **_tls_** (object): options relating to TLS when fetching resources over `https`.
      * **_certificate_authorities_** (list of objects): the list of additional certificate authorities (in addition to the system authorities) to be used for TLS verification when fetching over `https`. All certificate authorities must have a unique `source`.
        * **source** (string): the URL of the certificate bundle (in PEM format). With Ignition &ge; 2.4.0, the bundle can contain multiple concatenated certificates. Supported schemes are `http`, `https`, `s3`, `tftp`, and [`data`][rfc2397]. Note: When using `http`, it is advisable to use the verification option to ensure the contents haven't been modified.
        * **_verification_** (object): options related to the verification of the certificate.
          * **_hash_** (string): the hash of the certificate, in the form `<type>-<value>` where type is sha512.
* **_storage_** (object): describes the desired state of the system's storage devices.
  * **_disks_** (list of objects): the list of disks to be configured and their options. Every entry must have a unique `device`.
    * **device** (string): the absolute path to the device. Devices are typically referenced by the `/dev/disk/by-*` symlinks.
    * **_wipe_table_** (boolean): whether or not the partition tables shall be wiped. When true, the partition tables are erased before any further manipulation. Otherwise, the existing entries are left intact.
    * **_partitions_** (list of objects): the list of partitions and their configuration for this particular disk. Every partition must have a unique `number`, or if 0 is specified, a unique `label`.
      * **_label_** (string): the PARTLABEL for the partition.
      * **_number_** (integer): the partition number, which dictates it's position in the partition table (one-indexed). If zero, use the next available partition slot.
      * **_size_mib_** (integer): the size of the partition (in mebibytes). If zero, the partition will be made as large as possible.
      * **_start_mib_** (integer): the start of the partition (in mebibytes). If zero, the partition will be positioned at the start of the largest block available.
      * **_type_guid_** (string): the GPT [partition type GUID][part-types]. If omitted, the default will be 0FC63DAF-8483-4772-8E79-3D69D8477DE4 (Linux filesystem data).
      * **_guid_** (string): the GPT unique partition GUID.
      * **_wipe_partition_entry_** (boolean) if true, Ignition will clobber an existing partition if it does not match the config. If false (default), Ignition will fail instead.
      * **_should_exist_** (boolean) whether or not the partition with the specified `number` should exist. If omitted, it defaults to true. If false Ignition will either delete the specified partition or fail, depending on `wipePartitionEntry`. If false `number` must be specified and non-zero and `label`, `start`, `size`, `guid`, and `typeGuid` must all be omitted.
  * **_raid_** (list of objects): the list of RAID arrays to be configured. Every RAID array must have a unique `name`.
    * **name** (string): the name to use for the resulting md device.
    * **level** (string): the redundancy level of the array (e.g. linear, raid1, raid5, etc.).
    * **devices** (list of strings): the list of devices (referenced by their absolute path) in the array.
    * **_spares_** (integer): the number of spares (if applicable) in the array.
    * **_options_** (list of strings): any additional options to be passed to mdadm.
  * **_filesystems_** (list of objects): the list of filesystems to be configured. `path`, `device`, and `format` all need to be specified. Every filesystem must have a unique `device`.
    * **path** (string): the mount-point of the filesystem while Ignition is running relative to where the root filesystem will be mounted. This is not necessarily the same as where it should be mounted in the real root, but it is encouraged to make it the same.
    * **device** (string): the absolute path to the device. Devices are typically referenced by the `/dev/disk/by-*` symlinks.
    * **format** (string): the filesystem format (ext4, btrfs, xfs, vfat, or swap).
    * **_wipe_filesystem_** (boolean): whether or not to wipe the device before filesystem creation, see [the documentation on filesystems](https://coreos.github.io/ignition/operator-notes/#filesystem-reuse-semantics) for more information.
    * **_label_** (string): the label of the filesystem.
    * **_uuid_** (string): the uuid of the filesystem.
    * **_options_** (list of strings): any additional options to be passed to the format-specific mkfs utility.
  * **_files_** (list of objects): the list of files to be written. Every file, directory and link must have a unique `path`.
    * **path** (string): the absolute path to the file.
    * **_overwrite_** (boolean): whether to delete preexisting nodes at the path. `contents` must be specified if `overwrite` is true. Defaults to false.
    * **_contents_** (object): options related to the contents of the file.
      * **_compression_** (string): the type of compression used on the contents (null or gzip). Compression cannot be used with S3.
      * **_source_** (string): the URL of the file contents. Supported schemes are `http`, `https`, `tftp`, `s3`, and [`data`][rfc2397]. When using `http`, it is advisable to use the verification option to ensure the contents haven't been modified. If source is omitted and a regular file already exists at the path, Ignition will do nothing. If source is omitted and no file exists, an empty file will be created. Mutually exclusive with `inline`.
      * **_inline_** (string): the contents of the file. Mutually exclusive with `source`.
      * **_verification_** (object): options related to the verification of the file contents.
        * **_hash_** (string): the hash of the config, in the form `<type>-<value>` where type is `sha512`.
    * **_append_** (list of objects): list of contents to be appended to the file. Follows the same stucture as `contents`
      * **_compression_** (string): the type of compression used on the contents (null or gzip). Compression cannot be used with S3.
      * **_source_** (string): the URL of the contents to append. Supported schemes are `http`, `https`, `tftp`, `s3`, and [`data`][rfc2397]. When using `http`, it is advisable to use the verification option to ensure the contents haven't been modified. Mutually exclusive with `inline`.
      * **_inline_** (string): the contents to append. Mutually exclusive with `source`.
      * **_verification_** (object): options related to the verification of the appended contents.
        * **_hash_** (string): the hash of the config, in the form `<type>-<value>` where type is `sha512`.
    * **_mode_** (integer): the file's permission mode. If not specified, the permission mode for files defaults to 0644 or the existing file's permissions if `overwrite` is false, `contents` is unspecified, and a file already exists at the path.
    * **_user_** (object): specifies the file's owner.
      * **_id_** (integer): the user ID of the owner.
      * **_name_** (string): the user name of the owner.
    * **_group_** (object): specifies the group of the owner.
      * **_id_** (integer): the group ID of the owner.
      * **_name_** (string): the group name of the owner.
  * **_directories_** (list of objects): the list of directories to be created. Every file, directory, and link must have a unique `path`.
    * **path** (string): the absolute path to the directory.
    * **_overwrite_** (boolean): whether to delete preexisting nodes at the path. If false and a directory already exists at the path, Ignition will only set its permissions. If false and a non-directory exists at that path, Ignition will fail. Defaults to false.
    * **_mode_** (integer): the directory's permission mode. If not specified, the permission mode for directories defaults to 0755 or the mode of an existing directory if `overwrite` is false and a directory already exists at the path.
    * **_user_** (object): specifies the directory's owner.
      * **_id_** (integer): the user ID of the owner.
      * **_name_** (string): the user name of the owner.
    * **_group_** (object): specifies the group of the owner.
      * **_id_** (integer): the group ID of the owner.
      * **_name_** (string): the group name of the owner.
  * **_links_** (list of objects): the list of links to be created. Every file, directory, and link must have a unique `path`.
    * **path** (string): the absolute path to the link
    * **_overwrite_** (boolean): whether to delete preexisting nodes at the path. If overwrite is false and a matching link exists at the path, Ignition will only set the owner and group. Defaults to false.
    * **_user_** (object): specifies the symbolic link's owner.
      * **_id_** (integer): the user ID of the owner.
      * **_name_** (string): the user name of the owner.
    * **_group_** (object): specifies the group of the owner.
      * **_id_** (integer): the group ID of the owner.
      * **_name_** (string): the group name of the owner.
    * **target** (string): the target path of the link
    * **_hard_** (boolean): a symbolic link is created if this is false, a hard one if this is true.
* **_systemd_** (object): describes the desired state of the systemd units.
  * **_units_** (list of objects): the list of systemd units.
    * **name** (string): the name of the unit. This must be suffixed with a valid unit type (e.g. "thing.service"). Every unit must have a unique `name`.
    * **_enabled_** (boolean): whether or not the service shall be enabled. When true, the service is enabled. When false, the service is disabled. When omitted, the service is unmodified. In order for this to have any effect, the unit must have an install section.
    * **_mask_** (boolean): whether or not the service shall be masked. When true, the service is masked by symlinking it to `/dev/null`.
    * **_contents_** (string): the contents of the unit.
    * **_dropins_** (list of objects): the list of drop-ins for the unit. Every drop-in must have a unique `name`.
      * **name** (string): the name of the drop-in. This must be suffixed with ".conf".
      * **_contents_** (string): the contents of the drop-in.
* **_passwd_** (object): describes the desired additions to the passwd database.
  * **_users_** (list of objects): the list of accounts that shall exist. All users must have a unique `name`.
    * **name** (string): the username for the account.
    * **_password_hash_** (string): the hashed password for the account.
    * **_ssh_authorized_keys_** (list of strings): a list of SSH keys to be added as an SSH key fragment at `.ssh/authorized_keys.d/ignition` in the user's home directory. All SSH keys must be unique.
    * **_uid_** (integer): the user ID of the account.
    * **_gecos_** (string): the GECOS field of the account.
    * **_home_dir_** (string): the home directory of the account.
    * **_no_create_home_** (boolean): whether or not to create the user's home directory. This only has an effect if the account doesn't exist yet.
    * **_primary_group_** (string): the name of the primary group of the account.
    * **_groups_** (list of strings): the list of supplementary groups of the account.
    * **_no_user_group_** (boolean): whether or not to create a group with the same name as the user. This only has an effect if the account doesn't exist yet.
    * **_no_log_init_** (boolean): whether or not to add the user to the lastlog and faillog databases. This only has an effect if the account doesn't exist yet.
    * **_shell_** (string): the login shell of the new account.
    * **_system_** (bool): whether or not this account should be a system account. This only has an effect if the account doesn't exist yet.
  * **_groups_** (list of objects): the list of groups to be added. All groups must have a unique `name`.
    * **name** (string): the name of the group.
    * **_gid_** (integer): the group ID of the new group.
    * **_password_hash_** (string): the hashed password of the new group.
    * **_system_** (bool): whether or not the group should be a system group. This only has an effect if the group doesn't exist yet.

[part-types]: http://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_type_GUIDs
[rfc2397]: https://tools.ietf.org/html/rfc2397
