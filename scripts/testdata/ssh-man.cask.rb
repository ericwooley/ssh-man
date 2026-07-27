cask "ssh-man" do
  version "1.6.0"
  sha256 "1a46c45b0900c933aa167389f50a370b77c01947ed203ee3cfd2d0fe04dd20ef"

  url "https://github.com/ericwooley/ssh-man/releases/download/1.6.0/ssh-man.dmg"
  name "SSH Man"
  desc "Manage your SSH tunnels and SOCKS5 proxies."
  homepage "https://github.com/ericwooley/ssh-man"

  app "ssh-man.app"

  zap trash: [
    "~/Library/Application Support/ssh-man",
    "~/Library/Preferences/com.wails.ssh-man.plist",
    "~/Library/Preferences/tech.moonpixels.ssh-man.plist",
  ]
end
