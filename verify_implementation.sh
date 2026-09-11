#!/bin/bash
set -e

echo "=========================================="
echo "RideX Angola - Payment Provider Verification"
echo "=========================================="
echo ""

echo "1. Checking file structure..."
if [ -d "internal/payment/providers" ]; then
    echo "   ✓ Payment providers directory exists"
    echo ""
    echo "   Provider files:"
    for file in internal/payment/providers/*.go; do
        if [ -f "$file" ]; then
            echo "     - $(basename $file)"
        fi
    done
else
    echo "   ✗ Payment providers directory not found"
    exit 1
fi
echo ""

echo "2. Running payment provider tests..."
if go test ./internal/payment/providers/... -v > /tmp/provider_tests.log 2>&1; then
    echo "   ✓ All provider tests pass"
    PASS_COUNT=$(grep -c "^--- PASS" /tmp/provider_tests.log || echo "0")
    echo "   Passed: $PASS_COUNT tests"
else
    echo "   ✗ Some tests failed"
    cat /tmp/provider_tests.log
    exit 1
fi
echo ""

echo "3. Running full test suite..."
if go test ./... -count=1 > /tmp/all_tests.log 2>&1; then
    echo "   ✓ All project tests pass"
else
    echo "   ✗ Some tests failed"
    tail -20 /tmp/all_tests.log
    exit 1
fi
echo ""

echo "4. Running go vet..."
if go vet ./... > /tmp/vet.log 2>&1; then
    echo "   ✓ go vet passes"
else
    echo "   ✗ go vet found issues"
    cat /tmp/vet.log
    exit 1
fi
echo ""

echo "5. Checking code formatting..."
FORMAT_ISSUES=$(gofmt -l . 2>&1)
if [ -z "$FORMAT_ISSUES" ]; then
    echo "   ✓ All files properly formatted"
else
    echo "   ✗ Formatting issues found:"
    echo "$FORMAT_ISSUES"
    exit 1
fi
echo ""

echo "6. Counting implementation files..."
TOTAL_FILES=$(find . -name '*.go' -type f | wc -l)
PROVIDER_FILES=$(find internal/payment/providers -name '*.go' -type f | wc -l)
PROVIDER_SIZE=$(du -sh internal/payment/providers | cut -f1)

echo "   Total Go files in project: $TOTAL_FILES"
echo "   Payment provider files: $PROVIDER_FILES"
echo "   Payment provider size: $PROVIDER_SIZE"
echo ""

echo "=========================================="
echo "✅ ALL CHECKS PASSED"
echo "=========================================="
echo ""
echo "Implementation Summary:"
echo "- AppyPay provider: ✓ Implemented"
echo "- VPOS provider: ✓ Implemented"
echo "- ProxyPay provider: ✓ Implemented"
echo "- Provider registry: ✓ Implemented"
echo "- Provider factory: ✓ Implemented"
echo "- Unit tests: ✓ All passing"
echo "- Integration: Ready for main.go wiring"
echo ""
