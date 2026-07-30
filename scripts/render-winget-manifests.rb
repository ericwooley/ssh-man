#!/usr/bin/env ruby

require "fileutils"

CHANNEL, VERSION, SHA256, DESTINATION = ARGV

unless ARGV.length == 4
  abort "Usage: render-winget-manifests.rb <stable|experimental> <version> <sha256> <destination>"
end

SEMVER_PATTERN = /\A(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\z/
SHA256_PATTERN = /\A[0-9a-f]{64}\z/
MANIFEST_VERSION = "1.12.0"

CHANNELS = {
  "stable" => {
    package_identifier: "EricWooley.SSHMan",
    package_name: "SSH Man",
    short_description: "A desktop SSH connection manager with tunnels, SFTP, commands, and URL routing.",
    installer_name: "ssh-man-windows-amd64-installer.exe",
    product_code: "tech.moonpixels.ssh-man",
  },
  "experimental" => {
    package_identifier: "EricWooley.SSHMan.Experimental",
    package_name: "SSH Man Experimental",
    short_description: "The experimental release channel for SSH Man.",
    installer_name: "ssh-man-experimental-windows-amd64-installer.exe",
    product_code: "tech.moonpixels.ssh-man.experimental",
  },
}.freeze

channel = CHANNELS.fetch(CHANNEL) do
  abort "Unsupported WinGet channel: #{CHANNEL}"
end
abort "Version must be plain semantic versioning: #{VERSION}" unless VERSION.match?(SEMVER_PATTERN)
abort "SHA256 must be 64 lowercase hexadecimal characters" unless SHA256.match?(SHA256_PATTERN)

if File.exist?(DESTINATION) || File.symlink?(DESTINATION)
  abort "Destination already exists: #{DESTINATION}"
end

package_identifier = channel.fetch(:package_identifier)
package_name = channel.fetch(:package_name)
installer_url =
  "https://github.com/ericwooley/ssh-man/releases/download/#{VERSION}/#{channel.fetch(:installer_name)}"
release_url = "https://github.com/ericwooley/ssh-man/releases/tag/#{VERSION}"
schema_base = "https://aka.ms/winget-manifest"

version_manifest = <<~YAML
  # yaml-language-server: $schema=#{schema_base}.version.#{MANIFEST_VERSION}.schema.json

  PackageIdentifier: #{package_identifier}
  PackageVersion: #{VERSION}
  DefaultLocale: en-US
  ManifestType: version
  ManifestVersion: #{MANIFEST_VERSION}
YAML

locale_manifest = <<~YAML
  # yaml-language-server: $schema=#{schema_base}.defaultLocale.#{MANIFEST_VERSION}.schema.json

  PackageIdentifier: #{package_identifier}
  PackageVersion: #{VERSION}
  PackageLocale: en-US
  Publisher: Eric Wooley
  PublisherUrl: https://github.com/ericwooley
  PublisherSupportUrl: https://github.com/ericwooley/ssh-man/issues
  Author: Eric Wooley
  PackageName: #{package_name}
  PackageUrl: https://github.com/ericwooley/ssh-man
  License: Apache-2.0 with Commons Clause
  LicenseUrl: https://github.com/ericwooley/ssh-man/blob/main/LICENSE.md
  ShortDescription: #{channel.fetch(:short_description)}
  Description: Manage SSH connections, tunnels, SOCKS5 proxies, remote files, commands, and routed browser sessions.
  Tags:
    - browser
    - cli
    - sftp
    - socks5
    - ssh
    - tunnel
  ReleaseNotesUrl: #{release_url}
  ManifestType: defaultLocale
  ManifestVersion: #{MANIFEST_VERSION}
YAML

installer_manifest = <<~YAML
  # yaml-language-server: $schema=#{schema_base}.installer.#{MANIFEST_VERSION}.schema.json

  PackageIdentifier: #{package_identifier}
  PackageVersion: #{VERSION}
  Platform:
    - Windows.Desktop
  MinimumOSVersion: 10.0.17763.0
  InstallerType: nullsoft
  Scope: machine
  InstallModes:
    - interactive
    - silent
  UpgradeBehavior: install
  Installers:
    - Architecture: x64
      InstallerUrl: #{installer_url}
      InstallerSha256: #{SHA256}
      AppsAndFeaturesEntries:
        - DisplayName: #{package_name}
          Publisher: Eric Wooley
          DisplayVersion: #{VERSION}
          ProductCode: #{channel.fetch(:product_code)}
  ManifestType: installer
  ManifestVersion: #{MANIFEST_VERSION}
YAML

begin
  FileUtils.mkdir_p(DESTINATION)
  File.write(File.join(DESTINATION, "#{package_identifier}.yaml"), version_manifest)
  File.write(File.join(DESTINATION, "#{package_identifier}.locale.en-US.yaml"), locale_manifest)
  File.write(File.join(DESTINATION, "#{package_identifier}.installer.yaml"), installer_manifest)
rescue StandardError
  FileUtils.rm_rf(DESTINATION)
  raise
end

puts "Rendered #{CHANNEL} WinGet manifests for #{package_identifier} #{VERSION}: #{DESTINATION}"
