"""Tests for the log module."""

from unittest.mock import Mock, patch

from kodit.log import _from_sysfs, get_stable_mac_str


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


def test_from_sysfs_handles_non_directory_interfaces() -> None:
    """Test that _from_sysfs handles non-directory entries gracefully."""
    # Test the specific case where bonding_masters is a file, not a directory
    with patch("kodit.log.Path") as mock_path_class:
        # Create mock for Path("/sys/class/net")
        mock_base = Mock()
        mock_base.is_dir.return_value = True
        mock_path_class.return_value = mock_base

        # Mock interface paths
        mock_bonding_masters = Mock()
        mock_bonding_masters.is_dir.return_value = False  # This is a file
        mock_bonding_masters.name = "bonding_masters"

        # Mock base.iterdir() to return only the file interface
        mock_base.iterdir.return_value = [mock_bonding_masters]

        # The function should skip the file interface and return empty list
        result = _from_sysfs()

        # Should return empty list since bonding_masters is not a directory
        assert result == []


def test_from_sysfs_base_directory_not_exists() -> None:
    """Test that _from_sysfs returns empty list when /sys/class/net doesn't exist."""
    with patch("kodit.log.Path") as mock_path_class:
        mock_base = Mock()
        mock_base.is_dir.return_value = False
        mock_path_class.return_value = mock_base

        result = _from_sysfs()
        assert result == []
