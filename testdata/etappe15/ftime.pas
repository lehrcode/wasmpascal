program Dateizeit;
{ GetFTime, PackTime und UnpackTime. Ohne WASMPASCAL_UTC_OFFSET gilt UTC
  für Datum und Dateizeit gleichermaßen. }
uses Dos;
var
  F: file;
  T, P: LongInt;
  DT, D2: DateTime;
  J, M, Tg, W: Word;
begin
  Assign(F, 'x.dat');
  Rewrite(F, 1);
  Close(F);
  Reset(F, 1);
  GetFTime(F, T);
  WriteLn('DosError ', DosError);
  Close(F);
  UnpackTime(T, DT);
  GetDate(J, M, Tg, W);
  WriteLn('Datum passt: ', (DT.Year = J) and (DT.Month = M) and (DT.Day = Tg));
  PackTime(DT, P);
  WriteLn('PackTime zurueck: ', P = T);
  DT.Year := 2026; DT.Month := 11; DT.Day := 27;
  DT.Hour := 14; DT.Min := 58; DT.Sec := 31;
  PackTime(DT, P);
  WriteLn(P);
  UnpackTime(P, D2);
  WriteLn(D2.Year, '-', D2.Month, '-', D2.Day, ' ', D2.Hour, ':', D2.Min, ':', D2.Sec);
  GetFTime(F, T);
  WriteLn('geschlossen: DosError ', DosError)
end.
