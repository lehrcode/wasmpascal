program Reals;
var
  r, s: Real;
  e: Single;
  i: Integer;
begin
  r := 1.5;
  WriteLn(r);
  WriteLn(-r);
  WriteLn(0.0);
  WriteLn(r:8:2, '|', r:0:3, '|', -r:6:1, '|', 2.5:0:0);
  WriteLn(1234.5678:12);
  WriteLn(1E10);
  WriteLn(1.5E-5:10);
  s := 10 / 4;
  WriteLn(s:0:2);
  i := 7;
  r := i;
  r := r / 2 + i * 0.5;
  WriteLn(r:0:2);
  WriteLn(Sqrt(2):0:6, ' ', Sqr(1.5):0:2, ' ', Abs(-2.25):0:2);
  WriteLn(Trunc(3.7), ' ', Trunc(-3.7), ' ', Round(2.5), ' ', Round(-2.5), ' ', Round(2.4));
  e := 0.1;
  WriteLn(e:0:10);
  WriteLn(Pi:0:5);
  if (r > 6.9) and (r < 7.1) then WriteLn('Vergleich ok');
  WriteLn(1 / 3 * 3 = 1)
end.
