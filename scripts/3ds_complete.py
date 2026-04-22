#!/usr/bin/env python3
"""
Complete 3DS authentication by visiting the redirect URL with a headless browser.
Waits for the browser to eventually redirect back to the returnURL.

Usage: python3 scripts/3ds_complete.py <redirect_url> <return_url> [timeout_seconds]
Exit 0 if redirected to returnURL, 1 otherwise.
Prints the final URL to stdout (last line).
"""
import sys
import time


def main():
    if len(sys.argv) < 3:
        print("Usage: 3ds_complete.py <redirect_url> <return_url> [timeout_seconds]", file=sys.stderr)
        sys.exit(1)

    redirect_url = sys.argv[1]
    return_url_prefix = sys.argv[2]
    timeout_s = int(sys.argv[3]) if len(sys.argv) > 3 else 120

    from playwright.sync_api import sync_playwright

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        # Grant geolocation permission — 3DS simulator (sim-acs.higotw.com.tw) requests it
        context = browser.new_context(
            user_agent=(
                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/120.0.0.0 Safari/537.36"
            ),
            viewport={"width": 1280, "height": 720},
            geolocation={"latitude": 22.3193, "longitude": 114.1694},
            permissions=["geolocation"],
        )
        page = context.new_page()

        final_url = ""

        def on_response(response):
            nonlocal final_url
            if response.url.startswith(return_url_prefix):
                final_url = response.url

        page.on("response", on_response)

        try:
            print("Navigating to 3DS URL...", file=sys.stderr)
            try:
                page.goto(redirect_url, wait_until="domcontentloaded", timeout=30000)
            except Exception:
                pass  # Navigation interrupted by redirects is OK

            # Poll for returnURL redirect
            deadline = time.time() + timeout_s
            while time.time() < deadline:
                if final_url:
                    break
                try:
                    cur = page.url
                    if cur.startswith(return_url_prefix):
                        final_url = cur
                        break
                except Exception:
                    pass

                # Try clicking submit/continue buttons on 3DS challenge pages
                for selector in [
                    "button[type='submit']", "input[type='submit']",
                    "#submitBtn", "#submit",
                    "button:has-text('Submit')", "button:has-text('Continue')",
                    "button:has-text('OK')", "button:has-text('Verify')",
                ]:
                    try:
                        el = page.locator(selector).first
                        if el.is_visible(timeout=500):
                            print(f"Clicking: {selector}", file=sys.stderr)
                            el.click()
                            time.sleep(3)
                            break
                    except Exception:
                        continue

                time.sleep(2)
                remaining = int(deadline - time.time())
                try:
                    print(f"Waiting... ({remaining}s left) URL: {page.url[:80]}", file=sys.stderr)
                except Exception:
                    print(f"Waiting... ({remaining}s left)", file=sys.stderr)

            if final_url:
                print(f"3DS completed. Final URL: {final_url}", file=sys.stderr)
                print(final_url)
            else:
                try:
                    cur = page.url
                except Exception:
                    cur = "(unknown)"
                print(f"3DS did not redirect within {timeout_s}s. Last URL: {cur}", file=sys.stderr)
                try:
                    page.screenshot(path="/tmp/3ds_debug.png")
                    print("Debug screenshot: /tmp/3ds_debug.png", file=sys.stderr)
                except Exception:
                    pass
                print(cur)
                sys.exit(1)

        except Exception as e:
            print(f"Error: {e}", file=sys.stderr)
            sys.exit(1)
        finally:
            browser.close()


if __name__ == "__main__":
    main()
