#!/usr/bin/env python3
"""
Test script for VQA service.

This script helps verify that the VQA service is working correctly
by sending test images and displaying the results.
"""

import requests
import sys
import time
from pathlib import Path


def check_health(base_url="http://localhost:8000"):
    """Check if the VQA service is healthy."""
    try:
        response = requests.get(f"{base_url}/health", timeout=5)
        return response.status_code == 200
    except requests.exceptions.RequestException:
        return False


def test_caption(image_path, base_url="http://localhost:8000"):
    """
    Test image captioning with a local image file.
    
    Args:
        image_path: Path to image file
        base_url: VQA service URL
    """
    if not Path(image_path).exists():
        print(f"Error: Image file not found: {image_path}")
        return False
    
    try:
        with open(image_path, 'rb') as f:
            image_data = f.read()
        
        print(f"Sending image: {image_path}")
        response = requests.post(
            f"{base_url}/caption",
            data=image_data,
            headers={'Content-Type': 'image/jpeg'},
            timeout=30
        )
        
        if response.status_code == 200:
            result = response.json()
            print(f"✓ Caption: {result['caption']}")
            return True
        else:
            print(f"✗ Error: {response.status_code} - {response.text}")
            return False
            
    except Exception as e:
        print(f"✗ Error: {e}")
        return False


def wait_for_service(base_url="http://localhost:8000", timeout=60):
    """Wait for the VQA service to become available."""
    print(f"Waiting for VQA service at {base_url}...")
    start_time = time.time()
    
    while time.time() - start_time < timeout:
        if check_health(base_url):
            print("✓ VQA service is ready!")
            return True
        time.sleep(2)
    
    print(f"✗ VQA service did not start within {timeout} seconds")
    return False


def main():
    """Main test function."""
    print("VQA Service Test")
    print("=" * 50)
    
    base_url = "http://localhost:8000"
    
    # Wait for service
    if not wait_for_service(base_url):
        sys.exit(1)
    
    # Test with provided image or create a simple test
    if len(sys.argv) > 1:
        image_path = sys.argv[1]
        success = test_caption(image_path, base_url)
    else:
        print("\nUsage: python test_service.py <image_path>")
        print("\nExample:")
        print("  python test_service.py test-image.jpg")
        print("\nTo test the service, provide an image file path.")
        success = check_health(base_url)
    
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
