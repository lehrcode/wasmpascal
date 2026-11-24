program Mathe;
var
  x: Real;
begin
  WriteLn(Sin(0):0:6, ' ', Sin(Pi / 6):0:6, ' ', Sin(Pi / 2):0:6, ' ', Sin(-Pi / 4):0:6);
  WriteLn(Cos(0):0:6, ' ', Cos(Pi / 3):0:6, ' ', Cos(Pi):0:6, ' ', Cos(10):0:6);
  WriteLn(ArcTan(1) * 4:0:10, ' ', ArcTan(-1):0:6, ' ', ArcTan(1E10):0:6, ' ', ArcTan(0.5):0:6);
  WriteLn(Exp(0):0:6, ' ', Exp(1):0:10, ' ', Exp(-1):0:6, ' ', Exp(10):0:3);
  WriteLn(Ln(1):0:6, ' ', Ln(Exp(1)):0:6, ' ', Ln(10):0:6, ' ', Ln(0.5):0:6);
  WriteLn(Sqrt(16):0:1, ' ', Sqrt(2):0:10, ' ', Int(-3.7):0:1, ' ', Frac(-3.75):0:2);
  x := 2;
  WriteLn(Exp(x * Ln(x)):0:6, ' ', Sin(100):0:6, ' ', Sqr(Sin(1)) + Sqr(Cos(1)):0:12)
end.
