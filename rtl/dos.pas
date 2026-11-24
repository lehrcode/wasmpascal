{ Unit Dos von WasmPascal: nur Datum und Uhrzeit (PLAN.md). Beide kommen
  aus der Uhr des Ziels (System._Clock) und sind UTC, weil WASI keine
  Zeitzone kennt. }
unit Dos;

interface

procedure GetDate(var Year, Month, Day, DayOfWeek: Word);
procedure GetTime(var Hour, Minute, Second, Sec100: Word);

implementation

{ Liefert Tage und Hundertstelsekunden seit dem 1. 1. 1970 (UTC). }
procedure Jetzt(var Tage, Hundertstel: LongInt);
var
  Ns, Sek: Real;
begin
  Ns := _Clock(0);
  Sek := Int(Ns / 1E9);
  Tage := Trunc(Int(Sek / 86400));
  Hundertstel := Trunc((Sek - Tage * 86400.0) * 100 + Int((Ns - Sek * 1E9) / 1E7))
end;

procedure GetDate(var Year, Month, Day, DayOfWeek: Word);
var
  Tage, H, Z, Era, Doe, Yoe, Doy, Mp, Y, M: LongInt;
begin
  Jetzt(Tage, H);
  DayOfWeek := (Tage + 4) mod 7;   { 1. 1. 1970 war ein Donnerstag; 0 = Sonntag }
  { Umrechnung nach H. Hinnant, "civil_from_days" }
  Z := Tage + 719468;
  Era := Z div 146097;
  Doe := Z - Era * 146097;
  Yoe := (Doe - Doe div 1460 + Doe div 36524 - Doe div 146096) div 365;
  Y := Yoe + Era * 400;
  Doy := Doe - (365 * Yoe + Yoe div 4 - Yoe div 100);
  Mp := (5 * Doy + 2) div 153;
  Day := Doy - (153 * Mp + 2) div 5 + 1;
  if Mp < 10 then
    M := Mp + 3
  else
    M := Mp - 9;
  if M <= 2 then
    Inc(Y);
  Year := Y;
  Month := M
end;

procedure GetTime(var Hour, Minute, Second, Sec100: Word);
var
  Tage, H: LongInt;
begin
  Jetzt(Tage, H);
  Hour := H div 360000;
  Minute := H div 6000 mod 60;
  Second := H div 100 mod 60;
  Sec100 := H mod 100
end;

end.
