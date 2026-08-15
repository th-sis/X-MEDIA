import 'package:flutter_test/flutter_test.dart';
import 'package:xmedia_client/theme/app_theme.dart';

void main() {
  test('theme builds without error', () {
    expect(kodiTheme(), isNotNull);
  });
}
