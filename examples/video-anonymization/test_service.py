#!/usr/bin/env python3
"""
Test script for Video Anonymization Service.

This script tests the anonymization service by sending test images
and verifying the responses.
"""

import requests
import base64
import sys
from pathlib import Path

SERVICE_URL = "http://localhost:8000"


def test_health():
    """Test the health endpoint."""
    print("Testing /health endpoint...")
    try:
        response = requests.get(f"{SERVICE_URL}/health")
        if response.status_code == 200:
            data = response.json()
            print(f"✓ Health check passed: {data}")
            return True
        else:
            print(f"✗ Health check failed: {response.status_code}")
            return False
    except Exception as e:
        print(f"✗ Health check error: {e}")
        return False


def test_anonymize(test_image_path, method='blur'):
    """Test the anonymization endpoint."""
    print(f"\nTesting /anonymize endpoint with {method} method...")
    
    if not Path(test_image_path).exists():
        print(f"✗ Test image not found: {test_image_path}")
        return False
    
    try:
        with open(test_image_path, 'rb') as f:
            image_data = f.read()
        
        params = {'method': method}
        if method == 'blur':
            params['blur_factor'] = '50'
        else:
            params['pixel_size'] = '15'
        
        response = requests.post(
            f"{SERVICE_URL}/anonymize",
            data=image_data,
            params=params,
            headers={'Content-Type': 'image/jpeg'}
        )
        
        if response.status_code == 200:
            # Save the result
            output_path = f"test_output_{method}.jpg"
            with open(output_path, 'wb') as f:
                f.write(response.content)
            print(f"✓ Anonymization successful, saved to {output_path}")
            return True
        else:
            print(f"✗ Anonymization failed: {response.status_code}")
            print(f"  Response: {response.text}")
            return False
            
    except Exception as e:
        print(f"✗ Anonymization error: {e}")
        return False


def test_anonymize_info(test_image_path):
    """Test the anonymization info endpoint."""
    print(f"\nTesting /anonymize/info endpoint...")
    
    if not Path(test_image_path).exists():
        print(f"✗ Test image not found: {test_image_path}")
        return False
    
    try:
        with open(test_image_path, 'rb') as f:
            image_data = f.read()
        
        response = requests.post(
            f"{SERVICE_URL}/anonymize/info",
            data=image_data,
            params={'method': 'blur'},
            headers={'Content-Type': 'image/jpeg'}
        )
        
        if response.status_code == 200:
            data = response.json()
            print(f"✓ Info endpoint successful:")
            print(f"  Faces detected: {data.get('faces_detected', 0)}")
            print(f"  Method: {data.get('method', 'unknown')}")
            print(f"  Status: {data.get('status', 'unknown')}")
            
            # Optionally save the base64 image
            if 'image' in data:
                image_bytes = base64.b64decode(data['image'])
                with open('test_output_info.jpg', 'wb') as f:
                    f.write(image_bytes)
                print(f"  Saved to test_output_info.jpg")
            
            return True
        else:
            print(f"✗ Info endpoint failed: {response.status_code}")
            print(f"  Response: {response.text}")
            return False
            
    except Exception as e:
        print(f"✗ Info endpoint error: {e}")
        return False


def main():
    """Run all tests."""
    print("=== Video Anonymization Service Test Suite ===\n")
    
    # Check if test image is provided
    if len(sys.argv) > 1:
        test_image = sys.argv[1]
    else:
        test_image = "test_image.jpg"
        print(f"No test image provided, using default: {test_image}")
        print("Usage: python3 test_service.py <path_to_test_image.jpg>\n")
    
    results = []
    
    # Test health endpoint
    results.append(("Health Check", test_health()))
    
    # Only run other tests if we have a test image
    if Path(test_image).exists():
        # Test anonymization with blur
        results.append(("Anonymize (blur)", test_anonymize(test_image, 'blur')))
        
        # Test anonymization with pixelate
        results.append(("Anonymize (pixelate)", test_anonymize(test_image, 'pixelate')))
        
        # Test info endpoint
        results.append(("Anonymize Info", test_anonymize_info(test_image)))
    else:
        print(f"\n⚠ Test image '{test_image}' not found - skipping image tests")
        print("  To test with an image, provide path as argument:")
        print(f"  python3 test_service.py <image.jpg>")
    
    # Print summary
    print("\n" + "=" * 50)
    print("Test Summary:")
    print("=" * 50)
    for test_name, passed in results:
        status = "✓ PASS" if passed else "✗ FAIL"
        print(f"{status}: {test_name}")
    
    total = len(results)
    passed = sum(1 for _, p in results if p)
    print(f"\nTotal: {passed}/{total} tests passed")
    
    return 0 if passed == total else 1


if __name__ == "__main__":
    sys.exit(main())
