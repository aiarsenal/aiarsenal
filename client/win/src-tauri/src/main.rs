// Prevents additional console window on Windows in release, AND ensures `WinMain` is the entry.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    aiarsenal_client_win_lib::run()
}
