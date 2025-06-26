"""Tests for the log module."""

from kodit.log import get_stable_mac_str


def test_get_stable_mac_str_is_consistent() -> None:
    """Ensure that the MAC address returned is stable across repeated calls."""
    first = get_stable_mac_str()
    second = get_stable_mac_str()

    # It should be identical for subsequent calls (cached results)
    assert first == second, (
        "get_stable_mac_str should return a consistent value across calls"
    )

    # It should be a valid 12-character lowercase hex string
    assert len(first) == 12
    assert all(c in "0123456789abcdef" for c in first), (
        "MAC string should be lowercase hexadecimal"
    )
