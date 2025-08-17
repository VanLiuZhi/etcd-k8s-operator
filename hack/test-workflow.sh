#!/bin/bash

# Test script for the new development workflow
# This script validates the image management and deployment process

set -e

echo "🧪 Testing Development Workflow..."

# Test 1: Show current image
echo "📋 Test 1: Current image information"
make show-image

# Test 2: Build new image
echo ""
echo "🔨 Test 2: Building new image"
make docker-build

# Test 3: Show updated image list
echo "📋 Test 3: Updated image list"
make show-image

# Test 4: Generate deployment files
echo ""
echo "📄 Test 4: Generating deployment files"
make deploy-files

# Test 5: Check deployment file content
echo ""
echo "🔍 Test 5: Checking deployment file"
if [ -f "all-deploy.yaml" ]; then
    echo "✅ Deployment file generated successfully"
    echo "📊 File size: $(wc -l < all-deploy.yaml) lines"
    echo "🏷️  Image in deployment:"
    grep "image:" all-deploy.yaml | head -1
else
    echo "❌ Deployment file not found"
    exit 1
fi

# Test 6: Validate image naming
echo ""
echo "🏷️  Test 6: Validating image naming"
CURRENT_IMG=$(make show-image 2>/dev/null | grep "Current image:" | cut -d' ' -f3)
if [[ $CURRENT_IMG =~ ^etcd-operator:dev-[0-9]{8}-[0-9]{6}$ ]]; then
    echo "✅ Image naming follows correct pattern: $CURRENT_IMG"
else
    echo "❌ Image naming pattern incorrect: $CURRENT_IMG"
    exit 1
fi

echo ""
echo "🎉 All tests passed!"
echo ""
echo "📋 Summary:"
echo "   ✅ Image management working correctly"
echo "   ✅ Timestamp-based tagging functional"
echo "   ✅ Deployment file generation successful"
echo "   ✅ Image naming pattern validated"
echo ""
echo "🚀 Ready for development workflow!"
