#!/usr/bin/env ruby

CHANNEL, VERSION, SHA256, SOURCE_PATH, DESTINATION_PATH = ARGV

unless ARGV.length == 5
  abort "Usage: update-homebrew-cask.rb <stable|experimental> <version> <sha256> <source> <destination>"
end

CHANNEL_TOKENS = {
  "stable" => ["ssh-man", "ssh-man@experimental"],
  "experimental" => ["ssh-man@experimental", "ssh-man"],
}.freeze
SEMVER_PATTERN = /\A(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\z/
SHA256_PATTERN = /\A[0-9a-f]{64}\z/

tokens = CHANNEL_TOKENS.fetch(CHANNEL) do
  abort "Unsupported Homebrew channel: #{CHANNEL}"
end
abort "Version must be plain semantic versioning: #{VERSION}" unless VERSION.match?(SEMVER_PATTERN)
abort "SHA256 must be 64 lowercase hexadecimal characters" unless SHA256.match?(SHA256_PATTERN)

def replace_exactly_once(content, pattern, replacement, label)
  matches = content.scan(pattern).length
  abort "Expected exactly one #{label}; found #{matches}" unless matches == 1

  content.sub(pattern, replacement)
end

content = File.read(SOURCE_PATH)
cask_token, conflicting_token = tokens

content = replace_exactly_once(
  content,
  /^cask "ssh-man(?:@experimental)?" do$/,
  %(cask "#{cask_token}" do),
  "ssh-man cask declaration",
)
content = replace_exactly_once(
  content,
  /^  version ".*"$/,
  %(  version "#{VERSION}"),
  "version stanza",
)
content = replace_exactly_once(
  content,
  /^  sha256 ".*"$/,
  %(  sha256 "#{SHA256}"),
  "SHA256 stanza",
)
content = replace_exactly_once(
  content,
  /^  url ".*"$/,
  %(  url "https://github.com/ericwooley/ssh-man/releases/download/#{VERSION}/ssh-man.dmg"),
  "URL stanza",
)

content.sub!(
  %(  desc "Manage your SSH tunnels and SOCKS5 proxies."),
  %(  desc "Manage your SSH tunnels and SOCKS5 proxies"),
)

macos_dependency = "  depends_on :macos"
unless content.include?(macos_dependency)
  homepage_pattern = /^  homepage ".*"$/
  content = replace_exactly_once(
    content,
    homepage_pattern,
    "#{content[homepage_pattern]}\n\n#{macos_dependency}",
    "homepage stanza",
  )
end

conflict_pattern = /^  conflicts_with cask: "ssh-man(?:@experimental)?"$/
conflict_stanza = %(  conflicts_with cask: "#{conflicting_token}")
conflict_matches = content.scan(conflict_pattern).length
abort "Expected at most one conflict stanza; found #{conflict_matches}" if conflict_matches > 1

if conflict_matches == 1
  content.sub!(/^  conflicts_with cask: "ssh-man(?:@experimental)?"$\n/, "")
end
content.sub!(macos_dependency, "#{conflict_stanza}\n#{macos_dependency}")

app_stanza = %(  app "ssh-man.app")
abort "Missing expected app stanza" unless content.include?(app_stanza)

binary_stanza = %(  binary "\#{appdir}/ssh-man.app/Contents/MacOS/ssh-man")
unless content.include?(binary_stanza)
  content.sub!(app_stanza, "#{app_stanza}\n#{binary_stanza}")
end

File.write(DESTINATION_PATH, content)
